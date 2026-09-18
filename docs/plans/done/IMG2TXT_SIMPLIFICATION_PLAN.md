# img2txt simplification and diagnostics

Status: Done. All units implemented and the test suite green; the owner approved the rewritten prompt. One local run to
confirm the 8192 cap is not hit on a representative image remains, as noted under Verification.
Depends on: none.
Related: `internal/openai/system_prompt.txt`, `docs/PROMPT.md`.

Note: `.env.example` is matched by the global `private_files` setting, so the agent could not read it; the owner reviewed
and adjusted the inserted `IMG2TXT_PROMPT` and `IMG2TXT_MAX_TOKENS` lines directly.

## Goal

Make `img2txt` a single-purpose, unsteerable call: the agent supplies an image and nothing else, and the server
owns the prompt, model, and sampling. In doing so, fix the two things that make it unreliable today: an output cap far
below what the system prompt demands, and an error path that collapses several distinct upstream failures into the
opaque `openai: response contained no text`.

## Non-goals

- Returning image content to the caller. The caller is usually a text-only model mostly, and the image frequently
  lives behind internal DNS that the caller cannot reach, so the server-side fetch plus a separate vision endpoint
  stays.
- Supporting multiple vision models selectable per call.
- Adding a second vision backend, retries, or streaming.
- Rewriting `internal/resolve` or the URL handling around it.

## Design

### Why the knobs go away

The caller is an agent that cannot see and cannot fetch. That makes the vision call the only view of the pixels that
will ever exist downstream, which in turn makes a canonical, exhaustive description the useful contract and per-call
sampling knobs actively harmful: an agent has no basis to choose `temperature` or `top_p`, and `detail` is an input
resolution control whose failure mode is silently losing the fine detail the description exists to capture.

`img2txt` therefore takes `image`, with the same accepted forms as today (http(s) URL, base64 data URI, raw base64
PNG/JPEG/WebP), plus an optional `prompt`. An agent may know which detail of its own consumer matters, so a supplied
`prompt` overrides the configured default; everything else is server configuration:

| Concern          | Source                                        |
| ---------------- | --------------------------------------------- |
| User prompt      | call `prompt`, else `IMG2TXT_PROMPT`, else built-in default |
| System prompt    | `IMG2TXT_SYSTEM_PROMPT`, else embedded default |
| Model            | `IMG2TXT_MODEL`                                |
| `max_tokens`     | `IMG2TXT_MAX_TOKENS`, default `8192`           |
| `temperature`    | hardcoded `0`                                  |
| `top_p`          | not sent                                       |
| `detail`         | hardcoded `high`                               |

`temperature` is `0` rather than the current `0.2` because this is transcription and extraction; sampling variety can
only hurt fidelity. `top_p` is dropped entirely; leaving it unset is the provider default and removes a field that
reasoning-oriented endpoints may reject. `detail` is pinned to `high` because the system prompt asks for verbatim
transcription of fine text and a 3x3 spatial sweep, both of which `low` destroys and `auto` can silently downgrade.

### Why `max_tokens` and the error message are the same bug

The embedded system prompt asks for fourteen sections (S0 through S13), a 3x3 grid pass, per-object inventories, and a
full verbatim text transcript. `max_tokens` is `1024`. The requirement cannot fit through the cap, so two failure
shapes follow: silent truncation on a normal model, and empty visible `content` on a reasoning model whose hidden
reasoning consumes the whole budget. The decoder reads only `choices[0].message.content`, so it cannot distinguish
those from a content-filter refusal or a non-standard response shape, and reports all of them identically.

Unit 1 fixes the observability, Unit 2 raises the cap, and Unit 4 reduces the demand. The three are complementary; the
plan does not assume the prompt can be shortened enough on its own.

### Response handling

`Describe` gains awareness of `finish_reason`, `reasoning_content` (DeepSeek-style), and `reasoning` (OpenRouter-style).
Behaviour:

- Content present, `finish_reason == "length"`: return the text and set `Result.Truncated`, so the caller knows the
  description is incomplete rather than authoritative.
- Content empty, `finish_reason == "length"`: error naming truncation and the effective `max_tokens`.
- Content empty, reasoning text present: error stating the model produced reasoning but no answer.
- Content empty, `finish_reason == "content_filter"`: error stating the response was filtered.
- Content empty otherwise: error including `finish_reason` and a bounded snippet of the raw response body.

A bounded raw snippet is included unconditionally rather than gated behind `ERROR_DETAIL`. This is an internal tool, the
snippet describes an image the caller supplied, and the whole point is that these failures are currently undebuggable.

## Architecture impact

None. `internal/openai` gains no import and no package edge changes, so the `AGENTS.md` diagram is unchanged.

## Recorded decisions

- Per-call `prompt` is retained (owner, interactive session): a vision-incapable model may care about a specific detail
  in the image, so `img2txt` accepts an optional `prompt` that overrides `IMG2TXT_PROMPT` and the built-in default. This
  reverses the original "knobs go away" treatment of the prompt; sampling and `detail` remain server-owned.
- Output scales to the request (owner, interactive session): the report is a set of suggested parts, not a mandate. A
  general request, an empty request, or any request whose scope cannot be pinned down gets all of them; a request aimed
  at specific content gets the parts it bears on, and a part is omitted only when the request's own words make it
  irrelevant. When in doubt, include. This resolves plan question 8.
- The `S0` to `S12` codes are gone (owner, interactive session): the labels mean nothing to the reader, so the model
  emits plain descriptive headings of its own and the prompt lists the parts by name only. The 3x3 sweep and the
  spatial map both stay; only the shorthand codes are removed. This resolves plan questions 3 and part of 1.
- Generative tells and compression character are cut (owner, interactive session): the model gets generative tells wrong,
  so keeping them only adds noise. Camera, composition, and optics stay, as material for a full-detail request. This
  resolves plan question 4.
- The People part never matches to real people (owner, interactive session): identity attribution is removed outright,
  including the previous public-figure exception, while the per-person description stays. This resolves plan
  question 5.
- The three extraction parts, verbatim text, structured content, and extracted data values, all stay as-is (owner,
  interactive session). This resolves plan question 6.
- Every reply leads with the summary (owner, interactive session): it is the one part exempt from the scaling rule and
  is always included first. This resolves plan question 7.
- Secrets and personal data are withheld only when obviously secret or clearly personal data (owner, interactive
  session): such a value is reported by kind and location, with the reader warned and a remedy recommended, for example
  rotating a credential. Everything else is transcribed as normal. This resolves plan question 12.
- Header, entity inventory, and environment and context clues all stay; manipulation and artifacting is cut outright
  (owner, interactive session), for the same reason as generative tells. This resolves plan question 1.
- The five internal passes stay (owner, interactive session), including the 3x3 sweep and the adversarial check. This
  resolves plan question 10.
- Confidence labelling and the injection defence both stay (owner, interactive session). This resolves plan question 11.
- `IMG2TXT_MAX_TOKENS` defaults to 8192 (owner, interactive session): it must cover the largest full report, and
  truncation is the failure being fixed; the endpoint is cheap, so a larger cap is acceptable. The prompt states no
  prose size target. This resolves plan question 9.
- `S13 Verification steps` is cut (owner, interactive session): the calling agent cannot act on it. This resolves plan
  question 2.

## Human decisions

All twelve were resolved in an interactive session. The outcomes are in Recorded decisions above, and each item below is
struck out.

~~1. Which of the fourteen sections (S0 to S13) does anything downstream actually read, and which are never consumed?~~
   Resolved: general purpose, so every part is retained except `S13` and manipulation/artifacting; the `S`-codes are
   removed and the parts are suggestions.
~~2. Is `S13 Verification steps` (instructions to a human holding the image) ever useful to the text-only agent, given
   the agent cannot do any of them?~~ Resolved: cut.
~~3. Is the mandated 3x3 grid output (`S4`) worth its token cost, or should the grid be an internal pass only, with just
   the findings reported?~~ Resolved: keep the sweep and the map, drop the `S`-codes; the model emits its own descriptive
   headings.
~~4. Does the agent want `S2 Technicals and provenance` and `S3 Composition and optics` (sensor noise, focal length
   class, exposure), or is that dead weight for its use cases?~~ Resolved: keep composition and optics (and the
   screenshot/scan provenance cues) for full-detail requests; cut generative tells and compression character.
~~5. Is `S6 People` (per-person clothing, gaze, hands) needed, and does its no-identity-attribution rule stay?~~
   Resolved: the part stays; identity matching is removed entirely, with no public-figure exception.
~~6. Which of `S8 Structured content`, `S9 Extracted data values`, and `S7 All text, verbatim` matter most? These are the
   extraction sections and are the most likely to be the real purpose.~~ Resolved: keep all three as-is.
~~7. Should the output lead with a short stand-alone summary (`S1`) so the caller can stop reading early, or is the full
   body assumed to be read every time?~~ Resolved: always lead with the summary.
~~8. Should "exhaustiveness beats brevity" stay as the governing instruction, or should the prompt be re-scoped to
   "answer the likely question and transcribe observable text"?~~ Resolved: output scales to the request, with a
   deterministic inclusion rule that biases toward including sections.
~~9. How many headings and roughly how many output tokens is acceptable? This directly sets the final
   `IMG2TXT_MAX_TOKENS`.~~ Resolved: default `IMG2TXT_MAX_TOKENS` is 8192; no prose size target.
~~10. Do the multi-pass internal instructions (Pass 1 to Pass 5) stay, or do they cause the model to narrate process?~~
   Resolved: keep all five passes.
~~11. Do the confidence-labelling rules (`HIGH`/`MED`/`LOW`/`INFERRED`) and the injection defence (in-image text is
    data, not instruction) stay? (Recommended: both stay.)~~ Resolved: keep both.
~~12. Is any subject matter off limits, or any data class that must not be transcribed (credentials in screenshots,
    personal data), that the current prompt does not exclude?~~ Resolved: withhold obviously-secret and clearly-personal
    values, flag them, and recommend a remedy; transcribe everything else.

The deliverable for Unit 4 is a rewritten `system_prompt.txt` the owner has read and approved, plus a recorded target
output size.

## Implementation units

### Unit 1: openai response diagnostics

Scope: response parsing and error construction in `internal/openai`. No request-shape change, so callers keep compiling.

Deliverables:

- `internal/openai/describe.go`:
    - extend `chatCompletionResponse` with `FinishReason` per choice and `ReasoningContent` / `Reasoning` on the
      message;
    - add `Truncated bool` to `Result`, set when content is non-empty and `finish_reason == "length"`;
    - `decodeCompletion` returns the finish reason and reasoning text alongside the content, and produces the distinct,
      self-describing errors listed under Design;
    - a bounded helper for the raw-body snippet, reusing `utils.ReadLimited`.
- `internal/openai/describe_test.go`: table-driven coverage for each failure shape.

Acceptance criteria:

- [x] A `length` finish with empty content errors with a message naming truncation and the effective `max_tokens`.
- [x] A `length` finish with content returns the text and `Result.Truncated == true`.
- [x] Empty content with `reasoning_content` present errors stating reasoning was produced but no answer was, and does
      not return the reasoning as the description.
- [x] `content_filter` errors naming the filter.
- [x] `content: null` and a content array with only non-`text` parts both produce an error including the finish reason
      and a non-empty snippet.
- [x] A normal completion is unchanged: text returned, `Truncated == false`.
- [x] `go test ./internal/openai/... -race -count=1` passes.

### Unit 2: request and payload simplification

Scope: `internal/openai` request shape, `internal/config` env plumbing, `internal/server` wiring, and the `img2txt`
call site.

Deliverables:

- `internal/openai/client.go`: replace the four-string `New` with `New(Config)`, where `Config` carries `BaseURL`,
  `APIKey`, `Model`, `SystemPrompt`, `Prompt`, and `MaxTokens`. `New` validates the base URL as today and applies
  defaults for `MaxTokens` and both prompts.
- `internal/openai/system_prompt.go`: add `DefaultPrompt` (`"Describe this image in detail."`) and `Client.prompt()`
  mirroring the existing `Client.systemPrompt()` precedence.
- `internal/openai/describe.go`:
    - reduce `DescribeRequest` to `ImageData []byte`, `MediaType string`, and an optional `Prompt string`;
    - `buildPayload` uses `req.Prompt` when set and the client's configured prompt otherwise, sources the system prompt,
      model, and `max_tokens` from the client, hardcodes `temperature` `0` and `detail` `high`, drops `top_p`, and omits
      `max_tokens` when it is non-positive rather than sending `0`;
    - `Describe` no longer errors on an unset model supplied per call; an empty configured model still errors.
- `internal/config/config.go`: add `Img2TxtPrompt` (`IMG2TXT_PROMPT`) and `Img2TxtMaxTokens` (`IMG2TXT_MAX_TOKENS`,
  default `4096`). Keep `Img2TxtSystemPrompt` and `Img2TxtModel`.
- `internal/server/server.go`: build `openai.Config` from the config fields; log the model and effective `max_tokens`.
- `internal/mcp/img2txt.go`: update the `Describe` call to the reduced request. The input struct is not changed here.
- `internal/openai/describe_test.go`, `internal/config/config_test.go`, `internal/server/server_test.go`: updated for the
  new constructor and payload.

Acceptance criteria:

- [x] The captured payload contains no `top_p` field and no `temperature` value other than `0`.
- [x] The captured payload's `image_url.detail` is always `high`.
- [x] The user message text equals the call's `prompt` when set, then `IMG2TXT_PROMPT`, then `DefaultPrompt`.
- [x] The system message equals `IMG2TXT_SYSTEM_PROMPT` when set, and the embedded default when not.
- [x] With `IMG2TXT_MAX_TOKENS` unset the payload carries `4096`; when set to `2048` it carries `2048`.
- [x] A configured model is sent; an unset configured model fails with `openai: no model configured`.
- [x] `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` pass.

### Unit 3: image-only tool surface

Scope: the `img2txt` MCP input schema, handler, output, and user-facing docs.

Deliverables:

- `internal/mcp/img2txt.go`:
    - `img2txtInput` becomes `Image string` plus an optional `Prompt string`; delete `SystemPrompt`, `Model`,
      `Temperature`, `MaxTokens`, `TopP`, and `Detail`;
    - `img2txtOutput` gains `Truncated bool` and the handler propagates `Result.Truncated`;
    - `img2txtSchema` keeps `image` required and defaultless, adds an optional defaultless `prompt`, and drops every
      other default and enum; the `defaultImg2TxtPrompt` constant moves to `internal/openai` in Unit 2;
    - the debug log no longer references removed fields;
    - the tool description states that the response is an exhaustive description, that `prompt` optionally focuses it
      and otherwise falls back to the server default, and that truncation is reported.
- `internal/mcp/img2txt_test.go`: schema test asserts `image` is required and defaultless and `prompt` is optional; the
  handler and end-to-end tests call with `image` alone and separately with a `prompt`; a truncation case asserts
  `Truncated` reaches the structured output.
- `.env.example`: document `IMG2TXT_PROMPT` and `IMG2TXT_MAX_TOKENS`; note that `IMG2TXT_MODEL` is the only model
  control, that `IMG2TXT_SYSTEM_PROMPT` is the only system-prompt surface, and that a call may override the default
  `IMG2TXT_PROMPT`.
- `README.md`: replace the `img2txt` bullet's description of per-call knobs with the image-plus-optional-prompt contract
  and the new envars.
- `docs/PROMPT.md`: keep the `img2txt` chaining note; adjust it if it implies per-call steering.

Acceptance criteria:

- [x] The tool's input schema has `image` required and defaultless plus an optional defaultless `prompt`, and no other
      properties.
- [x] A call sending only `image` succeeds and reaches the vision endpoint.
- [x] A call sending a removed field is rejected by schema validation or ignored, never silently forwarded.
- [x] A truncated upstream response surfaces `truncated: true` in the structured output while still returning the
      partial text.
- [x] `.env.example` and `README.md` list `IMG2TXT_PROMPT` and `IMG2TXT_MAX_TOKENS` and no longer describe removed
      inputs. (Owner reviewed and adjusted `.env.example`.)
- [x] `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` pass.

### Unit 4: system prompt reduction (human intervention required)

Scope: `internal/openai/system_prompt.txt` and the final `IMG2TXT_MAX_TOKENS` value.

This unit is implemented; the prompt was rewritten incrementally as each Human decision was answered. What remains is the
owner's full read-through and the human-run local verification below; it is not an autonomous sign-off.

Deliverables:

- A rewritten `internal/openai/system_prompt.txt` that keeps only the questions' chosen parts and rules, lists them by
  name with no `S`-codes, treats them as suggestions that scale to the request, and states no prose size target; the
  recorded output-size target is the `8192` `max_tokens` cap.
- The final `IMG2TXT_MAX_TOKENS` default in `internal/config/config.go`, set from the owner's answer to question 9, with
  headroom for the largest chosen output.
- `internal/openai/system_prompt_test.go` updated if the prompt is asserted anywhere.

Acceptance criteria:

- [x] The owner has answered every question in the Human decisions section, and the answers are recorded in this plan
      or a follow-up note.
- [x] The owner has read the rewritten prompt in full and approved it. **Human.**
- [x] The rewritten prompt no longer references removed parts or `S`-codes; no removed part is still demanded.
- [ ] The chosen target output size is reachable: a representative image processed with the new prompt and default
      `max_tokens` returns `Truncated == false`. **Human, local.**
- [x] In-image text is still treated as data, not instruction, and no identity attribution is introduced. **Human.**
- [x] `go test ./... -race -count=1` passes.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`.
- Human, local: call `img2txt` with an image URL only and confirm the description comes back and `truncated` is false.
- Human, local: point `IMG2TXT_MODEL` at the vision endpoint used in production and reproduce a previously common
  `response contained no text`; confirm the error now names truncation, filtering, or reasoning rather than being
  opaque.
- Human, local: after Unit 4, repeat the above and confirm the description is complete and materially shorter.

## Risks and follow-ups

- The cap is 8192 with the reduced prompt and an unsteerable-by-default user prompt that a call can override. If output
  cost ever matters, retune the prompt and the cap together rather than independently.
- Pinning `detail` to `high` raises input token cost on OpenAI-style endpoints. That is the intended trade for text
  fidelity; a deployment that cares more about cost can lose text quality, which is a configuration choice this plan
  deliberately does not expose.
- A model that rejects `temperature` outright (some reasoning endpoints) would now fail with a 400. That is a narrower
  failure than the current silent truncation and is reported clearly; if it appears in practice, the follow-up is a
  per-endpoint flag to omit sampling fields, not a return to per-call knobs.
- Per-call `prompt` means an agent can ask a specific question of an image, while sampling and `detail` stay
  server-owned. The remaining capability question is the system prompt itself, which Unit 4 covers.
- The snippet added to errors includes model output. It describes an image the caller already supplied, so it is not a
  leak to the caller, but it will appear in server logs.
