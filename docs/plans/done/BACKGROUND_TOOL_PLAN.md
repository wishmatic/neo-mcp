# Background Tool Plan

Status: Done
Depends on: none

## Goal

Add a `background` MCP tool that takes a single hex colour and an image, and returns that image composited onto a subtle
one-directional gradient built from the colour. Gradients and compositing are implementation steps inside this one call,
not tools of their own.

```jsonc
// give a bgkill or crop cut-out a coloured backdrop
{ "hex": "#4a6fa5", "image_url": "https://host/i/cutout.png" }
```

## Motivation

`bgkill` and `crop` produce images with transparent areas, and there is no way to give them a backdrop without sending
them through a model. A gradient background is pure arithmetic, so it belongs next to them as a local tool.

## Scope

In scope:

- One mandatory hex colour and one mandatory image URL, plus the shared `format`.
- A subtle linear ramp in linear light, from a darker to a lighter variant of the hex.
- A random ramp direction on every call. Direction is not an input, since it does not matter.
- The canvas size is inherited from the input image, so there is no size input.
- An opaque result: the gradient is the base and the image is drawn over it, so the image's own transparent areas show
  the gradient through.
- A light dither, because a subtle ramp band badly once quantised, and the default output is lossy WebP.
- Registration independent of Forge and NovelAI.

Out of scope:

- Separate `gradient` or `composite` tools.
- Direction, size, or ramp-strength inputs.
- Radial gradients, multi-stop gradients, or more than one colour.
- Transparency in the result.
- Scaling or placing the image: it is drawn over the gradient at its own size, aligned to its own origin.
- A generation backend of any kind.

## Design

### One call does both steps

The handler fetches and decodes the image, builds a gradient canvas the same size, draws the image over it with
`draw.Over`, and publishes the result through the same path the other image tools use.

### The ramp

The hex converts to linear light once. The light end scales the base by `1+Ramp` and the dark end by `1-Ramp`, so the
middle of the ramp stays the base colour, and each pixel picks a point on it from the projection of its centre onto the
angle, normalised across the canvas corners. Working in linear light keeps the hue steady and avoids the grey middle an
sRGB blend gives. `Ramp` is `0.15`, a gentle but visible span.

### The dither

Before quantisation each channel gets triangular noise of about one least-significant bit, which turns the banding a
subtle ramp would otherwise show, especially after the lossy encode, into fine grain.

### Package layout

`internal/background` is a leaf: standard library only (`fmt`, `image`, `image/color`, `image/draw`, `math`,
`math/rand/v2`, `strconv`, `strings`), in the spirit of `internal/crop`.

```mermaid
flowchart LR
    mcp["internal/mcp"] --> background["internal/background"]
```

`internal/mcp` also keeps its `internal/crop` and `internal/imgfmt` arrows, and reuses `crop` and `imgfmt` for nothing
here; this tool only needs `imgfmt` to decode and encode.

## Implementation units

### Unit 1: the `internal/background` package

Deliverables: `internal/background/color.go`, `internal/background/background.go`,
`internal/background/background_test.go`.

Acceptance criteria:

- [x] `ParseHex` accepts `#rrggbb`, `rrggbb`, and the three-digit shorthands `#rgb` and `rgb`, in either case, and
      returns an opaque `color.NRGBA`; anything else is an error naming the value.
- [x] `RandomAngle` returns a value in `[0, 2π)` and varies with the source, and `Compose` with a fixed angle and a
      seeded dither source is deterministic.
- [x] `Compose` returns an image of the source's size, fully opaque, with the source drawn over the gradient.
- [x] With a fixed angle the ramp is monotonic: the pixels at the two extremes along the direction differ, the far end
      is lighter than the near end, and a transparent source leaves the gradient visible.
- [x] An opaque source comes back pixel-identical, since an opaque draw over any base is the source.
- [x] The ramp stays near the base colour: the mid-canvas pixel is within a small delta of the hex.

### Unit 2: the `background` tool

Deliverables: `internal/mcp/background.go`, `internal/mcp/background_test.go`, `internal/mcp/server.go`,
`internal/mcp/server_test.go`, `internal/mcp/annotations_test.go`.

Acceptance criteria:

- [x] A `background` call with `hex` and `image_url` returns a URL text block and an image block, in the user's and the
      assistant's audience, in the requested `format`.
- [x] The result has the input image's dimensions, proven with an image of known size and a transparent centre.
- [x] The tool is registered with no Forge and no NovelAI configured, and `TestToolRegistration` expects `background`
      in every case.
- [x] The tool carries the image-generation annotations, and `TestToolAnnotations` covers it.
- [x] The schema marks `hex` and `image_url` required and sets the `format` enum and default.
- [x] A malformed hex, a fetch failure, and an undecodable image each return an error prefixed `background:`.

### Unit 3: documentation

Deliverables: `README.md`, `docs/PROMPT.md`, `AGENTS.md`.

Acceptance criteria:

- [x] `README.md` describes `background` as local, backend-free, sized from its input, and random in direction.
- [x] `docs/PROMPT.md` tells the agent to pass a cut-out's URL and one hex colour to give it a backdrop.
- [x] `AGENTS.md`'s diagram gains the `internal/mcp` to `internal/background` arrow.

## Verification

Per unit, `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1`, as CI runs them.

Human check, marked because it needs a deployment: call `background` on a `bgkill` or `crop` cut-out and confirm the
gradient sits behind the transparent areas and reads as smooth rather than banded at full size.
