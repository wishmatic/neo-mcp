# Image output consolidation into `present`

Status: Done
Depends on: none
Related: `done/IMAGE_CONTENT_AUDIENCE_PLAN.md`, which introduced the audience annotations this plan relocates.

## Goal

Give image output presentation a single home in `internal/present`, so the two paths that build MCP image content share
one implementation instead of two near-copies:

- `internal/mcp/publish.go` builds `[text(url), image, ...]` for stored generation results.
- `internal/mcp/inline.go` builds `[text(caption), image]` for a fetched image.

## Non-goals

- Changing what the client sees: block order, captions, audiences, and the fail-soft notes stay as they are.
- Changing `imgfmt.Inline`: downscaling, WebP re-encoding, and the size cap are unchanged.
- Renaming or rewording the `inline` tool, its schema, or its documentation.

## Design

### Package boundary

`internal/present` becomes the presentation layer for MCP content and may import the MCP SDK. It must not import
`internal/mcp`, keeping the dependency one-way (`internal/mcp` -> `internal/present`), and it takes `internal/imgfmt`
as a leaf for image preparation. `AGENTS.md` records the amended rule and the new diagram edge.

### API

`internal/present/image.go`:

- `RoleUser` and `RoleAssistant`: the two audience role strings, in one place.
- `type AttachmentFailure struct { Index int; Err error }`.
- `func StoredImages(images [][]byte, urls []string, forAssistant bool) ([]mcp.Content, []AttachmentFailure)`: emits
  `[text(url), image, ...]`, calling `imgfmt.Inline` for each image. An image that cannot be prepared keeps its URL line
  and yields a failure; the failure's text is the placeholder note. `forAssistant` decides the image audience.
- `func InlineImage(url string, image imgfmt.InlineImage) []mcp.Content`: a caption plus the image, always in the
  assistant and user audience because reading it is the call's purpose.
- `func InlineImageFailure(url string, err error) []mcp.Content`: the fail-soft note for a fetched image that could not
  be prepared.

### Logging

`StoredImages` is pure: it returns failures so `internal/mcp` keeps logging each at warn with the image index.
`InlineImageFailure` is also pure and its caller keeps the log line.

## Implementation units

### Unit 1: `internal/present` image output API

Deliverables:

- `internal/present/image.go`, `internal/present/image_test.go`.

Acceptance criteria:

- [x] `StoredImages` pairs every URL with its image and annotates the image `["user"]`, or `["assistant", "user"]` when
      `forAssistant`.
- [x] An unpreparable image leaves its URL text and a note, is reported as an `AttachmentFailure` at its 1-based index,
      and does not stop later images.
- [x] `StoredImages` emits the URL text alone when there is no image data for it.
- [x] `InlineImage` returns a caption naming the URL plus an image block always annotated `["assistant", "user"]`.
- [x] `InlineImageFailure` returns a single text note.
- [x] `go test ./internal/present/... -race -count=1` passes.

### Unit 2: `internal/mcp` adopts the API

Deliverables:

- `internal/mcp/publish.go`: drop `roleUser`, `roleAssistant`, `imageContent`, and `imageAudience`; call
  `present.StoredImages` and log its failures.
- `internal/mcp/inline.go`: call `present.InlineImage` and `present.InlineImageFailure`.
- `internal/mcp/publish_test.go`, `inline_test.go`, `provider_test.go`: audience assertions use the exported roles.

Acceptance criteria:

- [x] `internal/mcp` defines no image content or audience helper of its own.
- [x] `publishImages` still returns `[text(url), image, ...]` with the same audiences and fail-soft note.
- [x] The `inline` tool still returns its caption and a `["assistant", "user"]` image block.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 3: architecture documentation

Deliverables:

- `AGENTS.md`: the rule exempts `internal/present` as the presentation layer, while forbidding it from importing
  `internal/mcp`; the diagram gains `present -> imgfmt`.

Acceptance criteria:

- [x] The rule names `internal/present` as the package below `internal/server` allowed to import the MCP SDK besides
      `internal/mcp`.
- [x] The diagram matches the imports in the code.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

## Risks and follow-ups

- `internal/present` now depends on the MCP SDK, so it is no longer a dependency-free leaf. This is the intended,
  documented architecture change.
