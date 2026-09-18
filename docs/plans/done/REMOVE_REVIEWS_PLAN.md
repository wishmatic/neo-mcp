# Remove the reviews feature

Status: Done. All units implemented and the test suite green. Two owner-run checks remain, listed under Verification.
Depends on: none.
Related: `internal/mcp/reviews.go`, `internal/mcp/reviews_schema.go`, `internal/present/review.go`, `internal/store/review.go`.

## Goal

Delete the model reviews feature entirely: the `add_review`, `get_reviews`, `get_review`, and `delete_review` tools, the
presentation and storage layers behind them, the `reviews` table and its migration, and every doc mention. The `examples`
feature and the SQLite database itself stay, since `get_examples` still uses both.

## Non-goals

- Removing the SQLite store or the `examples` feature.
- Dropping data from existing databases. See Recorded decisions.

## Design

The feature spans four layers, and each unit removes one so the tree keeps compiling after every unit:

| Unit | Layer                | Files                                              |
| ---- | -------------------- | -------------------------------------------------- |
| 1    | MCP adapter          | `internal/mcp/reviews*.go`, `internal/mcp/server.go` |
| 2    | Presentation         | `internal/present/review*.go`                        |
| 3    | Storage and schema   | `internal/store/review*.go`, `internal/store/store.go`, `internal/store/validate.go` |
| 4    | Documentation        | `README.md`, `docs/PROMPT.md`                        |

Removing from the top down means the intermediate states compile: Unit 1 removes the only callers of `present.Review*`,
Unit 2 removes the only callers of `store.Review*`, and Unit 3 removes the store types last.

### What stays

- `store.Client`, `store.New`, `migrate`, `formatTime`, `parseTime`, and the `examples` table and index.
- `validateModel` and `MaxModelLength`, which `SaveExample` uses.
- `present` as a package: `anlas.go` and `example.go` are untouched.
- `handlers.store` and `Deps.Store`, which `get_examples` needs.

### What goes

- Review-only store constants: `MaxCommentLength`, `MaxPromptLength`, `MaxAgentCommentLength`, `MaxImageURLLength`.
- Review-only validators: `validateComment`, `validateOptionalText`, `validateImageURL`.
- The `reviewDetailColumns` migration, `addReviewDetailColumns`, and `hasColumn`, which exist only for review columns.

## Recorded decisions

- The `reviews` table is removed from `schema`, so fresh databases never create it. Existing databases keep an orphaned
  `reviews` table and its rows; no `DROP TABLE` is added, because silently destroying user data is worse than leaving an
  unused table behind. An owner who wants the data gone can delete the table or the database file directly.
- `newReviewStore`, the shared test helper that happens to live in `reviews_test.go`, is renamed `newTestStore` and moved
  into `examples_test.go`, the only file that still uses it.
- `textContent`, a second shared MCP test helper that also lived in `reviews_test.go`, moves to `examples_test.go`
  alongside `newTestStore` for the same reason.

## Implementation units

### Unit 1: MCP tool surface

Scope: the four review tools, their schemas, registration, and the two shared test helpers.

Deliverables:

- Delete `internal/mcp/reviews.go`, `internal/mcp/reviews_schema.go`, and `internal/mcp/reviews_test.go`.
- `internal/mcp/server.go`: drop the `if h.store != nil { registerReviews(srv, h) }` block. Keep `Deps.Store`, the
  `handlers.store` field, and the `internal/store` import, which `get_examples` uses.
- `internal/mcp/examples_test.go`: add `newTestStore` and `textContent` (the old helper bodies) and replace the eleven
  `newReviewStore` call sites.

Acceptance criteria:

- [x] `add_review`, `get_reviews`, `get_review`, and `delete_review` are no longer registered under any dependency set.
- [x] `internal/mcp` has no identifier referencing the reviews feature.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/mcp/... -race -count=1` pass.

### Unit 2: presentation layer

Scope: the Markdown renderers for reviews.

Deliverables:

- Delete `internal/present/review.go` and `internal/present/review_test.go`.
- No other file changes: `present.Review*` was called only from `internal/mcp`, removed in Unit 1.

Acceptance criteria:

- [x] `internal/present` contains no identifier referencing the reviews feature and still exposes the `anlas` and
      `example` renderers unchanged.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/present/... -race -count=1` pass.

### Unit 3: storage layer and schema

Scope: the review store API, the `reviews` table and its migration, the review-only validators and limits, and the store
tests that cover them.

Deliverables:

- Delete `internal/store/review.go` and `internal/store/review_test.go`.
- `internal/store/store.go`: remove the `reviews` table and `reviews_model_id_idx` index from `schema`; remove
  `reviewDetailColumns`; remove `addReviewDetailColumns` and its call in `migrate`; remove `hasColumn`.
- `internal/store/validate.go`: keep `validateModel`; delete `validateComment`, `validateOptionalText`, and
  `validateImageURL`; drop the now-unused `net/url` and `unicode` imports.
- `internal/store/store.go`: delete `MaxCommentLength`, `MaxPromptLength`, `MaxAgentCommentLength`, and
  `MaxImageURLLength`; keep `MaxModelLength`.
- `internal/store/store_test.go`: update `TestNewCreatesDatabaseAndSchema` to expect `examples` and assert no `reviews`
  table; update `TestNewReopensExistingDatabase` and `TestClosePreventsFurtherUse` to write and read an `examples` row
  instead of a review; delete `TestNewAddsReviewDetailColumnsToExistingDatabase`; rewrite
  `TestNewAddsMissingTablesToExistingDatabase` to start from a database that already contains a legacy `reviews` table,
  assert `examples` is created, and assert the legacy `reviews` row is untouched.
- `internal/server/server_test.go`: `TestShutdownClosesStore` reads through `RandomExamples` instead of writing a review.

Acceptance criteria:

- [x] A database created by `store.New` contains an `examples` table and no `reviews` table.
- [x] Opening a pre-existing database that still has a legacy `reviews` table succeeds, does not create a `reviews` table
      if absent, and does not modify or drop an existing one.
- [x] Writing and reading an example survives closing and reopening the database.
- [x] `internal/store` has no identifier referencing the reviews feature.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/store/... -race -count=1` pass.

### Unit 4: documentation

Scope: the two docs that describe the feature.

Deliverables:

- `README.md`: delete the `add_review` / `get_reviews` / `get_review` / `delete_review` bullet, and move the
  "Created automatically at `DB_PATH`" line to the `get_examples` bullet, since that is now the only database user.
- `docs/PROMPT.md`: delete the reviews bullet from "Model memory", leaving the `get_examples` bullet as the only item.

Acceptance criteria:

- [x] Neither doc names any review tool or the `reviews` table.
- [x] The `DB_PATH` behavior is still documented, now next to `get_examples`.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all pass.
- A repo-wide search for the review tool names and review store identifiers returns matches only in this plan.
- Human, local: start the server against an existing `/data` database that has a `reviews` table and confirm it boots and
  `get_examples` still works. **Human.**
- Human, local: confirm `.env.example` needs no change beyond any `DB_PATH` note that frames the database as review
  storage. **Human.**

## Risks and follow-ups

- The orphaned `reviews` table keeps whatever prompts and image URLs were recorded. That is a privacy question, not a
  correctness one; deleting the table is a one-line manual `DROP TABLE reviews` if wanted.
- `coverage.out` is an untracked local artifact and is now stale relative to the deleted files; it can be deleted or
  regenerated freely.
