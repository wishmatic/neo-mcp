# OpenAI-compatible image generation

## Goal

Add an `openai` image generation tool that speaks the OpenAI Images API (`POST /images/generations`) to any number of
configured endpoints, so that a caller can generate through OpenAI itself or through any gateway that copies its
protocol, and can name which one it wants. The first configured endpoint is the default.

The tool is named for the protocol, not for a vendor: any endpoint that speaks it can be configured, and none of them
is special.

## Scope

In scope:

- A new `internal/openai` package: the image call, and parsing and selection of providers.
- `OPENAI_PROVIDERS` configuration, and wiring in `internal/server`.
- The `openai` MCP tool, including provider selection in its schema (enum, defaulting to the first).
- `resolve.Image.DataURL`, because these endpoints take inputs inline rather than as addresses to fetch.

Out of scope:

- Image edits as multipart (`/images/edits`); the inline data URL fields on `/images/generations` cover img2img,
  multi-image input, and masking.
- Model discovery; the caller names a model the way it names one for `forge` and `novelai`.

## Design

### Providers

`OPENAI_PROVIDERS` is a comma-separated list. An entry is `[name=]endpoint=key`:

- `cloud=https://api.openai.com/v1=sk-xxx` names a provider.
- `https://api.openai.com/v1=sk-xxx` takes its name from the host, so `api.openai.com`.
- A key may be left off, for an endpoint that asks for none, and the `Authorization` header is only sent when one is
  set.

Empty means the tool is not registered, the way an unset `NOVELAI_API_KEY` leaves `novelai` unregistered. A malformed
list fails startup rather than disabling the tool, and no error echoes a key.

### Errors versus naming

`GenerateImages(ctx, providerName, req)` resolves the name itself and falls back to the first provider, so the selection
rule sits in one place. The resolved name comes back in `ImageResult.Provider` for the output and the log line, which
is also what keeps the tool from resolving twice.

### Presentation

`openai` is registered against `c.publishImages` exactly as `forge` and `novelai` are: convert to the target format,
store, then present `[text(url), image]` per image with the shareable inline budget. An image the endpoint answers with
a URL instead of bytes is fetched through `resolve.Client.Fetch` first, so one code path handles storage, format
conversion, and presentation.

The output extends `generationOutput` with the provider, the requested model, and what the call cost.

## Implementation units

### 1. `internal/openai`

- `providers.go`: `Provider`, `ParseProviders`, endpoint parsing and normalisation, duplicate-name rejection.
- `client.go`: `Client`, `New`, the JSON POST helper, provider lookup, `ProviderNames`.
- `images.go`: `ImageRequest`, `imageResponse`, `GeneratedImage`, `ImageResult`, `GenerateImages`, the tolerant base64
  decode (`data:` prefix, unpadded standard encoding).
- Tests: parse table, selection, request wire shape, both response shapes, error responses, no key leakage.

### 2. Configuration and wiring

- `Config.OpenAIProviders` from `OPENAI_PROVIDERS`.
- `server.New` parses the list, builds the client when it is non-empty, and logs the provider names (never the keys) at
  startup.
- `Clients.OpenAI`, registered in `registerTools` when set.
- Architecture diagram in `AGENTS.md` gains `openai` (leaf: `utils` only) and the `mcp --> openai` edge.
- `README.md` and `docs/PROMPT.md` gain the tool and the envar.

### 3. The `openai` tool

- `internal/mcp/openai.go`: input struct, schema (provider enum defaulting to the first, format, inline budget,
  generations, target, seed), handler, and the extended output struct.
- `internal/mcp/openai_images.go`: data URL resolution for `image`/`images`/`mask`, and reading a URL answer back into
  bytes.
- Tests: schema shape (enum, default, required), the call reaching the configured provider with the right body,
  provider selection and defaulting, unknown provider, the URL fallback answer, audience annotation, and that keys do
  not reach the logs.

## Acceptance criteria

All of these are implemented and covered by tests except where noted. The last item is the one a human still has to
confirm.

1. [x] `OPENAI_PROVIDERS` unset or empty registers no `openai` tool and needs no other configuration.
2. [x] A malformed entry (no endpoint, non-http scheme, no host, duplicate name) fails `server.New` with an error
   naming `OPENAI_PROVIDERS`.
3. [x] With several providers, a call that names one posts to that endpoint with that key; a call that names none posts
   to the first. An unknown name fails without any request.
4. [x] Provider names are matched case-insensitively.
5. [x] The request body carries the prompt, model, `n`, size, seed, and the inline inputs, and always asks for
   `response_format: b64_json`. Unset optional fields are absent from the JSON.
6. [x] An inline `b64_json` answer is decoded (including a `data:` prefix and missing padding) and stored; a `url`
   answer is fetched and stored; each is returned as its URL in text, an image block in the user and assistant
   audience, and its URL in `urls`.
7. [x] The stored image is the named output format, and the attached copy honours `inline_max_edge` and
   `inline_max_bytes`.
8. [x] A non-2xx answer and an answer with no images both fail the call with an error naming the tool and the provider,
   and store nothing.
9. [x] The structured output carries `provider`, `model`, `count`, `urls`, and `costUsd`.
10. [x] No API key or inline image bytes appear in any log line.

One thing changed while implementing: an unknown provider is refused by the schema's `provider` enum before the handler
runs, so the caller gets the list of configured names from the schema rather than from an error. The client's own "no
provider named" error still exists for calls that do not go through the schema, such as a name that differs only in
case.

## Verification

- [x] `go test ./...`, `go vet ./...`, and `gofmt -l .` are clean.
- [ ] The human check: one real call against a live endpoint, confirming the image comes back attached and that
  `costUsd` matches what the endpoint billed.
