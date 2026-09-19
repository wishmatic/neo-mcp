# Inline image returns

Status: Done
Depends on: none
Related: none.

## Goal

Add a second way for the image tools to hand results back to the caller. Every image-returning tool call must now pass
a mandatory `return_as` flag:

- `url` (the default mode): behaves exactly as today. The image is stored and a link is returned as text.
- `image`: the image is also returned inline as an MCP `image` content block, so a vision-capable model can see it.
  This is very token-expensive, so the flag's description warns callers that it is only worth using when the model can
  see images.

Inline returns follow the MCP contract that clients like LibreChat ingest into their own file store: raw base64 in the
block's `data`, a `mimeType` from `{image/png, image/jpeg, image/webp, image/gif}`, one block per image, and a short
text caption alongside the blocks.

## Non-goals

- Returning images inline by default, or by any server-wide setting. It is per-call only.
- Changing the stored-file behaviour: every call still uploads and still returns URLs in its structured output.
- Inlining the `examples` tool output. It returns remembered metadata and image URLs as a text document, not a
  generation result; inlining it would mean fetching and re-encoding arbitrary stored files per call.
- New environment configuration. The downscale target and size cap are fixed constants.
- GIF output. The encoder produces WebP, which is in the allowed media-type set.

## Design

### The flag

`return_as` is added to the shared generation inputs and to `bgkill`. It is a required property with an enum of `url`
and `image`. It deliberately has no schema default: the point of making it mandatory is that the caller cannot silently
omit it, and the property description is where the cost warning lives.

Handlers still treat an empty value as `url`. Schema validation rejects a missing value before the handler runs for
MCP callers, and the leniency keeps direct handler use and internal call sites working unchanged.

### Conversion for the wire

Inline images are always re-encoded to WebP, regardless of the call's `format`. WebP is in the MCP media-type set, keeps
transparency, and compresses screenshots and generated art well. `format` still controls the stored file and the
returned URL; the inline copy is a vision input, not the artifact.

Before encoding, the longest edge is capped at 1024 px (downscale only, never upscale) and the result is re-encoded at
decreasing quality until it fits 1 MiB. If it still does not fit, the image is halved and the quality ladder repeats,
down to 1x1. A pathological input that cannot fit returns an error, which becomes a text block, never bad base64.

Downscaling uses a premultiplied-alpha box average, so transparent regions do not bleed dark edges into the result.

### Content shape

`url` mode is unchanged: one `text` block per returned URL.

`image` mode returns:

1. one `text` block: image count plus the stored URLs, so chaining calls on a URL still works;
2. one `image` block per image, with raw bytes in `Data` (the SDK base64-encodes it on the wire) and
   `mimeType: image/webp`;
3. if an image cannot be encoded, a `text` block naming the image and the error, in place of its `image` block.

The `generationOutput` structured result is unchanged in both modes.

## Implementation units

### Unit 1: inline image preparation in `imgfmt`

Deliverables:

- `internal/imgfmt/inline.go`:
    - `type InlineImage struct { Data []byte; MediaType string }`;
    - `func Inline(data []byte, maxEdge, maxBytes int) (InlineImage, error)`, which decodes `data`, downscales to
      `maxEdge`, then encodes WebP at a decreasing quality ladder and halving scale until the output fits `maxBytes`;
    - `InlineMaxEdge = 1024` and `InlineMaxBytes = 1 << 20` constants;
    - a premultiplied-alpha box-average resampler used only for downscaling.
- `internal/imgfmt/inline_test.go`.

Acceptance criteria:

- [x] A source whose longest edge exceeds the cap comes back WebP with a longest edge at or under the cap, and the
      aspect ratio is preserved within a pixel.
- [x] A source already within the cap is not upscaled.
- [x] PNG, JPEG, WebP, and JXL inputs all produce `image/webp`, never `image/jpg` or `application/octet-stream`.
- [x] When a small byte budget is used, the encoded output is at or under it.
- [x] An impossible byte budget returns an error and never a partial or malformed image.
- [x] Non-image input returns an error with an `imgfmt:` prefix.
- [x] Transparent source regions stay transparent after downscaling.
- [x] `go test ./internal/imgfmt/... -race -count=1` passes.

### Unit 2: `return_as` flag and inline content blocks

Deliverables:

- `internal/mcp/shared.go`: `returnInput` with the required `return_as` property; `returnMode`, `parseReturnMode`, and
  `setReturnSchema`.
- `internal/mcp/txt2img.go`, `img2img.go`, `bgkill.go`: embed the flag, call `setReturnSchema` in each schema builder,
  parse the mode with a tool-prefixed error, pass it to `publishImages`, and log it at debug level.
- `internal/mcp/publish.go`: `publishImages` takes the mode; `url` mode keeps the current one-text-block-per-URL
  output; `image` mode builds the caption plus image blocks and swallows per-image encode failures into a text block.
- `internal/mcp/publish_test.go`, `shared_test.go`, `format_test.go`, `provider_test.go`, `examples_test.go`: updated
  call sites and new coverage.

Acceptance criteria:

- [x] `return_as` is required in the `txt2img`, `img2img`, and `bgkill` schemas, with an enum of `url` and `image` and
      no default.
- [x] A call that omits `return_as` is rejected by schema validation.
- [x] A `url` call returns exactly the URL text blocks it does today.
- [x] An `image` call returns a caption text block followed by one `image` block per image, each with a non-empty
      `image/webp` payload that decodes.
- [x] An `image` call still uploads and still reports the URLs in both the caption and the structured output.
- [x] An image that cannot be encoded yields a text block explaining the failure and does not fail the call.
- [x] An invalid `return_as` returns a tool-prefixed error before any generation happens.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 3: documentation

Deliverables:

- `README.md`: document the per-call flag, its default, and the cost of `image`; correct the claim that images are
  never returned inline.
- `docs/PROMPT.md`: instruct the agent to pass `return_as` on every image call and to prefer `url` unless it can see
  images and needs to.

Acceptance criteria:

- [x] The README no longer states that images are never returned inline and explains `return_as`.
- [x] `docs/PROMPT.md` names the flag, both values, and when to use each.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.
- Human, local: call `txt2img` with `return_as: "url"` and confirm the response is a URL as before; call it again with
  `return_as: "image"` from a vision-capable client and confirm the image is visible and the stored URL still resolves.
- Human, LibreChat: confirm an `image` result is attached to the tool message and visible to a vision model.

## Risks and follow-ups

- `image` mode multiplies the token cost of a call; the schema description and the prompt are the whole mitigation.
- The 1 MiB cap and 1024 px edge are fixed and may need to become tunable if a client has different limits.
- `examples` could grow an inline mode later; it is out of scope here.
