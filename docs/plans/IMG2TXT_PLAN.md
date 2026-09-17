# `img2txt` Implementation Plan

## Goal

Add an `img2txt` MCP tool that recognises an image and returns a textual response for it, backed by any OpenAI-compatible
`/chat/completions` endpoint (base URL, API key, model, plus optional tuning). The image input may be an http(s) URL, a
base64 data URI, or raw base64 data, in PNG, JPEG, or WebP.

## Scope

In scope:

- New `img2txt` MCP tool.
- Reuse of the existing resource fetching stack (`internal/imageresolve`) so URLs, shortener redirects, and Garagefront
  S3 reads keep working with no duplicated logic.
- New OpenAI-compatible vision client.
- Config, wiring, docs, and tests.

Out of scope:

- Multiple images per call (single image input).
- Streaming responses.
- Provider-specific extensions beyond `temperature`, `max_tokens`, `top_p`, and image `detail`.
- Output uploads or S3 interaction (the result is text, not an image).

## Design

### Reuse strategy

The existing `imageresolve.Resolver` already resolves an http(s) URL to bytes, following redirects and short-circuiting
Garagefront URLs by reading them straight from S3 (`Resolver.Fetch`). The new image input also accepts base64, which is
not a fetch. Rather than duplicating fetch logic, we extend `imageresolve` with a higher-level `Resolve` that normalises
all three input forms (URL, data URI, raw base64) into `Image{Data, MediaType}`. `Resolve` calls the existing `Fetch`
for URL inputs, so the redirect and Garagefront behaviour is shared, and `Fetch` itself is left unchanged for
`img2img`/`bgkill`.

The OpenAI client lives in a new `internal/img2txt` package and only knows about raw image bytes plus a media type. It
does not import `imageresolve`, so resolution and provider concerns stay separate.

### Data flow

```mermaid
flowchart TD
    A[Agent] -->|image, prompt, tuning| B[MCP img2txt tool]
    B --> C{imageresolve.Resolve}
    C -->|http(s) URL| D[Resolver.Fetch: redirects + Garagefront S3]
    C -->|data URI or raw base64| E[decode base64]
    D --> F[sniff png/jpeg/webp]
    E --> F
    F --> G[img2txt.Describe]
    G -->|POST /chat/completions| H[OpenAI-compatible endpoint]
    H --> G
    G --> B
    B -->|text + structured output| A
```

### Package layout

| Path | Change | Purpose |
| --- | --- | --- |
| `internal/imageresolve/input.go` | New | `Image` type, media type constants, `Resolve` |
| `internal/imageresolve/input_test.go` | New | Tests for `Resolve` |
| `internal/img2txt/client.go` | New | `Client`, `New`, HTTP plumbing |
| `internal/img2txt/describe.go` | New | `DescribeRequest`, `Describe`, payload and response building |
| `internal/img2txt/describe_test.go` | New | Tests for the client |
| `internal/mcp/img2txt.go` | New | Tool registration, input schema, `runImg2Txt` handler |
| `internal/mcp/img2txt_test.go` | New | Handler and registration tests |
| `internal/mcp/server.go` | Edit | Accept and gate on `*img2txt.Client` |
| `internal/mcp/server_test.go` | Edit | Update `New` call |
| `internal/config/config.go` | Edit | New env vars |
| `internal/config/config_test.go` | Edit | Tests for new env vars |
| `internal/server/server.go` | Edit | Build client from config, pass to MCP |
| `.env.example` | Edit | Document new env vars |
| `README.md` | Edit | Add a feature bullet |

### Configuration

| Env var | Required | Default | Purpose |
| --- | --- | --- | --- |
| `IMG2TXT_BASE_URL` | No | empty | OpenAI-compatible base URL; tool is enabled only when set |
| `IMG2TXT_API_KEY` | No | empty | Sent as `Authorization: Bearer` only when non-empty |
| `IMG2TXT_MODEL` | No | empty | Default model; may be overridden per call |
| `IMG2TXT_TIMEOUT_SECONDS` | No | `180` | Per-call HTTP timeout for the vision endpoint |

`IMG2TXT_BASE_URL` should include any prefix the endpoint needs (for example `https://api.openai.com/v1`); the client
appends `/chat/completions`.

## Implementation Units

### Unit 1: Resolve image inputs in `imageresolve`

Add a unified resolution entry point that turns any supported image reference into bytes plus a media type.

Files:

- `internal/imageresolve/input.go` (new)
- `internal/imageresolve/input_test.go` (new)
- `internal/imageresolve/resolver.go` (no behaviour change; only adjust if a shared helper is extracted)

API:

```go
type Image struct {
    Data      []byte
    MediaType string
}

const (
    MediaPNG  = "image/png"
    MediaJPEG = "image/jpeg"
    MediaWebP = "image/webp"
)

func (r *Resolver) Resolve(ctx context.Context, input string) (Image, error)
```

Behaviour:

1. Trim surrounding whitespace. Empty input is an error.
2. Input beginning with `data:` is parsed as a data URI: the metadata must end in `;base64` (case-insensitive) and the
   payload is base64-decoded.
3. Input matching `utils.IsHTTP` is fetched with the existing `r.Fetch(ctx, input)`, preserving redirects and Garagefront
   S3 reads.
4. Any other input is treated as raw base64 and decoded.
5. The decoded bytes are sniffed with `http.DetectContentType`. The sniffed type wins when supported; otherwise the
   data URI's declared media type is used as a fallback when supported. A resulting type outside
   `{image/png, image/jpeg, image/webp}` is rejected with an error naming the detected type.
6. Base64 decoding accepts standard, unpadded standard, and URL-safe alphabets, and tolerates embedded whitespace.

Acceptance criteria:

- AC-1.1: For an http(s) input, `Resolve` returns exactly the same bytes as `Fetch`, including a multi-hop redirect
  chain and a redirect that lands on a Garagefront URL read from the object store.
- AC-1.2: Data URIs `data:image/png;base64,...`, `data:image/jpeg;base64,...`, and `data:image/webp;base64,...` decode
  to the raw image bytes.
- AC-1.3: Raw base64 input (no `data:` prefix) decodes for all three formats, including unpadded base64, URL-safe
  base64, and base64 containing newlines.
- AC-1.4: `Image.MediaType` is one of the three constants; when sniffing is inconclusive the data URI's declared
  supported media type is used.
- AC-1.5: Input whose sniffed or declared type is unsupported (for example GIF or BMP) returns an error that includes
  the offending type.
- AC-1.6: Empty input, malformed base64, and a `data:` URI without `;base64` each return a distinct error, all wrapped
  with an `imageresolve:` prefix.
- AC-1.7: Base64 and data URI inputs perform no network I/O; URL inputs do.
- AC-1.8: Existing `internal/imageresolve/resolver_test.go` passes unmodified, proving `Fetch` behaviour is unchanged.

### Unit 2: OpenAI-compatible vision client

Add `internal/img2txt` with a small client that sends a single-image chat completion request and returns the text.

Files:

- `internal/img2txt/client.go` (new)
- `internal/img2txt/describe.go` (new)
- `internal/img2txt/describe_test.go` (new)

API:

```go
type Client struct { /* baseURL, apiKey, defaultModel, http */ }

func New(baseURL, apiKey, defaultModel string, timeout time.Duration) (*Client, error)

type DescribeRequest struct {
    ImageData    []byte
    MediaType    string
    Prompt       string
    SystemPrompt string
    Model        string
    Temperature  float64
    MaxTokens    int
    TopP         float64
    Detail       string
}

type Result struct {
    Text  string
    Model string
}

func (c *Client) Describe(ctx context.Context, req DescribeRequest) (Result, error)
```

Behaviour:

- `New` trims trailing `/` from `baseURL`, requires a non-empty http(s) `baseURL` and a positive timeout, and returns a
  wrapped error otherwise.
- `Describe` resolves the model as request `Model`, else the client default, else an error.
- It builds `data:{mediaType};base64,{data}` from the image bytes and posts to `{baseURL}/chat/completions`.
- `Authorization: Bearer <apiKey>` is set only when the key is non-empty; `Content-Type` is always `application/json`.
- Messages are an optional `system` message (only when `SystemPrompt` is non-empty) followed by a `user` message whose
  content array holds a `text` part then an `image_url` part carrying the data URI and `detail`.
- Payload includes `temperature`, `max_tokens`, and `top_p`.
- Response text is read from `choices[0].message.content`, supporting both a plain string and an array of text parts.
- Non-2xx responses return an error containing the status and the upstream `error.message` when present, otherwise a
  limited body excerpt. Empty choices or empty text are errors.

Acceptance criteria:

- AC-2.1: `New` normalises a trailing slash so the request path is exactly `{baseURL}/chat/completions`; an empty or
  non-http(s) base URL and a non-positive timeout each return an error.
- AC-2.2: The request is a `POST` to `/chat/completions` with `Content-Type: application/json`; `Authorization:
  Bearer <key>` is present when the key is set and absent when it is empty.
- AC-2.3: The `image_url.url` value is `data:{MediaType};base64,{base64data}` and the part also carries
  `image_url.detail`; the `text` part carries `Prompt` and precedes the image part.
- AC-2.4: A `system` message is included only when `SystemPrompt` is non-empty.
- AC-2.5: `temperature`, `max_tokens`, and `top_p` are carried with the provided values in the JSON body.
- AC-2.6: Model resolution prefers the request model, falls back to the client default, and errors when neither is set;
  `Result.Model` reports the resolved model.
- AC-2.7: String content and array-of-text-parts content both parse to the expected text; empty content and malformed
  JSON return errors.
- AC-2.8: A non-2xx response surfaces the upstream `error.message` when present, and the call fails with a wrapped error
  rather than returning empty text.
- AC-2.9: `Describe` uses `http.NewRequestWithContext`, so a cancelled context aborts the call.
- AC-2.10: Tests use `httptest` to assert method, path, headers, and body, and cover both success shapes plus all error
  paths; no live network is used.

### Unit 3: Configuration

Add the four env vars and document them.

Files:

- `internal/config/config.go` (edit)
- `internal/config/config_test.go` (edit)
- `.env.example` (edit)

Acceptance criteria:

- AC-3.1: `Config` gains `Img2TxtBaseURL`, `Img2TxtAPIKey`, `Img2TxtModel`, and `Img2TxtTimeoutSeconds` with the env
  tags and defaults from the configuration table.
- AC-3.2: A test asserts `IMG2TXT_TIMEOUT_SECONDS` defaults to `180` when unset and honours an override, following the
  existing `S3_USE_PATH_STYLE` test style.
- AC-3.3: `.env.example` lists all four vars under an `img2txt (optional)` heading with a one-line comment each and a
  note that the base URL should include any prefix such as `/v1`.

### Unit 4: `img2txt` MCP tool

Register the tool, define its schema, and keep the handler logic independently testable.

Files:

- `internal/mcp/img2txt.go` (new)
- `internal/mcp/img2txt_test.go` (new)
- `internal/mcp/server.go` (edit)
- `internal/mcp/server_test.go` (edit)

Input schema:

| Field | Type | Required | Default | Notes |
| --- | --- | --- | --- | --- |
| `image` | string | yes | none | http(s) URL, base64 data URI, or raw base64 |
| `prompt` | string | no | `Describe this image in detail.` | What to ask about the image |
| `system_prompt` | string | no | none | Optional system instruction |
| `model` | string | no | none | Overrides the server default model |
| `temperature` | number | no | `0.2` | Sampling temperature |
| `max_tokens` | integer | no | `1024` | Response token cap |
| `top_p` | number | no | `1.0` | Nucleus sampling |
| `detail` | string | no | `auto` | Enum: `auto`, `low`, `high` |

Output:

```go
type img2txtOutput struct {
    Text  string `json:"text" jsonschema:"the model's textual response"`
    Model string `json:"model" jsonschema:"the model that produced the response"`
}
```

The handler is factored into `runImg2Txt(ctx, log, resolver, client, in) (img2txtOutput, error)` so it can be unit
tested directly. The registered closure logs the call, invokes `runImg2Txt`, and returns a `CallToolResult` whose content
is a single `TextContent` carrying `Text`.

Registration is gated on `img2txtClient != nil` and is independent of the `sdClient` gate that currently wraps the
three SD tools.

Acceptance criteria:

- AC-4.1: `New` and `registerTools` accept a `*img2txt.Client`; `img2txt` is registered when it is non-nil and is absent
  when it is nil. Absence is asserted by connecting an in-memory MCP client session and listing tools.
- AC-4.2: The registered tool is named `img2txt`; `image` is required with no default; `prompt`, `temperature`,
  `max_tokens`, `top_p`, and `detail` have the defaults in the table; `detail` has the enum `auto|low|high`; `model` and
  `system_prompt` have no default.
- AC-4.3: `runImg2Txt` resolves the image with `imageresolve.Resolve`, so URL inputs (including shortened and Garagefront
  URLs) and base64 inputs both reach the client as raw bytes with the correct media type.
- AC-4.4: `runImg2Txt` forwards `prompt`, `system_prompt`, `model`, and the tuning values to `Describe` and returns the
  resolved model in `img2txtOutput.Model`.
- AC-4.5: Image resolution failure returns `img2txt: resolve image: %w`; a client failure returns `img2txt: %w`; the
  registered closure surfaces these as tool errors.
- AC-4.6: An end-to-end test wires an `httptest` image server and an `httptest` OpenAI-compatible server into a real
  `img2txt.Client` and `Resolver`, calls the tool through an in-memory MCP session, and asserts the returned text, plus
  that the image bytes and prompt arrived correctly at the vision endpoint.
- AC-4.7: `internal/mcp/server_test.go` is updated to the new `New` signature, and `go test ./internal/mcp/...` passes.

### Unit 5: Server wiring

Build the client from config and hand it to the MCP server.

Files:

- `internal/server/server.go` (edit)

Acceptance criteria:

- AC-5.1: When `IMG2TXT_BASE_URL` is empty, no client is built, `img2txt` is not registered, and startup succeeds.
- AC-5.2: When `IMG2TXT_BASE_URL` is set, a client is built with the configured key, model, and timeout, and `img2txt`
  is registered.
- AC-5.3: An invalid base URL fails startup with a `build img2txt client: %w` error.
- AC-5.4: `IMG2TXT_API_KEY` remaining empty is valid (for keyless local endpoints) and does not block startup.
- AC-5.5: Enabling `img2txt` logs an info line with the base URL and default model; disabling it does not.

### Unit 6: Documentation

Files:

- `README.md` (edit)

Acceptance criteria:

- AC-6.1: The README features list gains one bullet describing `img2txt` and that it needs `IMG2TXT_BASE_URL`,
  `IMG2TXT_API_KEY`, and `IMG2TXT_MODEL`.
- AC-6.2: The README stays high level and points at `.env.example` for the remaining settings, matching the existing
  style.

## Validation

- `go build ./...`
- `go vet ./...`
- `go test ./... -race -count=1 -coverprofile=coverage.out`

All three must pass. No live network calls are required by any test; every external interaction is exercised with
`httptest` servers or in-memory transports.

## Decisions

- Base64 inputs are resolved to bytes and always sent to the vision endpoint as a `data:` URI, never passed through as a
  remote URL. This is required for Garagefront and shortener URLs, which the endpoint may not be able to reach, and it
  keeps a single code path for all input forms.
- The API key is optional because common OpenAI-compatible servers (Ollama, LM Studio) accept unauthenticated requests.
- The tool is enabled by `IMG2TXT_BASE_URL` alone, independent of the Stable Diffusion instance, so `img2txt` works even
  if Forge is not configured.
