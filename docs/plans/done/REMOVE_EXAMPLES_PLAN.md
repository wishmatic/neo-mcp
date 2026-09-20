# Remove the examples feature

Status: Done. All units implemented; `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` are green. One owner-run check remains, listed under Verification.
Depends on: none.
Related: `internal/mcp/examples.go`, `internal/present/example.go`, `internal/store/example.go`,
`docs/plans/done/REMOVE_REVIEWS_PLAN.md`.

## Goal

Delete the examples feature entirely: the `examples` MCP tool, the per-generation capture, the Markdown renderer, the
storage API, the `examples` table and its migration, the `EXAMPLES_ENABLED` setting, and every doc mention.

The SQLite store stays wired into `internal/server` (open at `DB_PATH`, close on shutdown), but after this change
nothing reads or writes any rows, and the store creates no tables. That leftover is deliberate: it is reported to the
owner rather than removed here.

## Non-goals

- Removing the SQLite store, the `DB_PATH` setting, or the database lifecycle wiring in `internal/server`.
- Dropping data from existing databases. See Recorded decisions.

## Design

The feature spans four layers. Each unit removes one so the tree keeps compiling after every unit:

| Unit | Layer                  | Files                                                                                                                                     |
| ---- | ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | MCP surface and config | `internal/mcp/examples*.go`, `internal/mcp/{server,handlers,txt2img,img2img,shared}.go`, `internal/config/*`, `internal/server/server.go` |
| 2    | Presentation           | `internal/present/example*.go`                                                                                                            |
| 3    | Storage and schema     | `internal/store/{example*.go,validate.go,store.go,store_test.go}`, `internal/server/server_test.go`                                       |
| 4    | Documentation          | `README.md`, `docs/PROMPT.md`, `.env.example`                                                                                             |

Removing from the top down keeps intermediate states compiling: Unit 1 removes the only callers of `present.Examples`
and `store.*Example*`; Unit 2 removes the renderer; Unit 3 removes the store code, the table, and the migration helpers
that only existed for it.

### What stays

- `store.Client`, `store.New`, `store.Close`, `store.Ping`, and `dsn`.
- The store lifecycle wiring in `internal/server`: `store.New(cfg.DBPath)`, the `database opened` log, `Server.store`,
  `Shutdown`'s `s.store.Close()`, and the `DB_PATH` setting.
- `publishImages` and the generation `nsfw` flag, which still place images under the `nsfw/` subdirectory.
- `present.Anlas` and the rest of `internal/present`.

### What goes

- The `examples` MCP tool, its input/output types, schema, and registration.
- `handlers.saveExamples` and its two call sites in `txt2img` and `img2img`.
- `Deps.Store` and `handlers.store`, which existed only for the tool and the capture.
- `ExamplesConfig`, `Deps.Examples`, `handlers.examplesConfig`, `config.Config.ExamplesEnabled`, and `EXAMPLES_ENABLED`.
- `present.Examples`, `jsonBlock`, and `indentJSON`.
- `store.ExampleMeta`, `store.Example`, `SaveExample`, `SaveExamples`, `RandomExamples`, `RandomExamplesByNSFW`,
  `randomExamples`, and `boolToInt`.
- The `examples` table and `examples_model_id_idx` index from `schema`, plus `exampleColumns`, `addMissingColumns`,
  `hasColumn`, `formatTime`, and `parseTime`, which served only the examples table. `migrate` collapses to a ping.
- `internal/store/validate.go`: `validateModel` and `MaxModelLength` were examples-only.

## Recorded decisions

- The `examples` table is removed from `schema`, so fresh databases never create it. Existing databases keep an orphaned
  `examples` table and its rows; no `DROP TABLE` is added, because silently destroying user data is worse than leaving an
  unused table behind. An owner who wants the data gone can delete the table or the database file directly.
- Because no tables are created any more, `migrate` is reduced to a `PingContext`; the schema transaction and the
  `addMissingColumns`/`hasColumn` helpers go with it.
- `store.Ping` is added so `internal/server`'s `TestShutdownClosesStore` can still observe that `Shutdown` closed the
  database, replacing its use of the deleted `RandomExamples`.
- Helpers defined in `internal/mcp/examples_test.go` (`examplesModel`, `newTestStore`, `textContent`, `callTxt2Img`,
  `callImg2Img`, `resultURL`, `storedQuery`) are used only by that file and are deleted with it. Shared helpers that live
  elsewhere (`newFakeStore`, `newFailingStore`, `zapNop`, `connectSession`, `newResolver`, `newForgeBackend`,
  `newNovelAIBackend`, `newInitImageURL`) stay.
- The generation `nsfw` flag stays: it still routes images to the `nsfw/` subdirectory. Its descriptions drop the
  "tags the saved example" clause.

## Implementation units

### Unit 1: MCP surface and configuration

Scope: the `examples` tool, the capture, the `nsfw` description text, the config flag, and the server wiring that feeds
them.

Deliverables:

- Delete `internal/mcp/examples.go` and `internal/mcp/examples_test.go`.
- `internal/mcp/server.go`: drop `Store` and `Examples` from `Deps`, drop `store: deps.Store` and
  `examplesConfig: deps.Examples` from `buildHandlers`, drop the `registerExamples` block from `registerTools`, and
  remove the now-unused `internal/store` import.
- `internal/mcp/handlers.go`: drop the `store` and `examplesConfig` fields and the now-unused `internal/store` import.
- `internal/mcp/txt2img.go` and `internal/mcp/img2img.go`: remove the `h.saveExamples(...)` call.
- `internal/mcp/shared.go`: reword the `NSFW` field description to drop "it tags the saved example".
- `internal/config/config.go`: remove `ExamplesEnabled`.
- `internal/config/config_test.go`: delete `TestLoadExamplesDefaults` and `TestLoadExamples`.
- `internal/server/server.go`: drop `Store: storeClient` and the `Examples: mcpServer.ExamplesConfig{...}` from the
  `Deps` literal. Keep `store.New`, the `database opened` log, `Server.store`, and `Shutdown`.

Acceptance criteria:

- [x] `examples` is not registered under any dependency set.
- [x] `internal/mcp` and `internal/config` contain no identifier referencing the examples feature.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/mcp/... ./internal/config/... -race -count=1` pass.

### Unit 2: presentation layer

Scope: the Markdown renderer for examples.

Deliverables:

- Delete `internal/present/example.go` and `internal/present/example_test.go`.
- No other file changes: `present.Examples` was called only from `internal/mcp`, removed in Unit 1.

Acceptance criteria:

- [x] `internal/present` contains no identifier referencing the examples feature and still exposes `Anlas` unchanged.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/present/... -race -count=1` pass.

### Unit 3: storage layer and schema

Scope: the examples store API, the `examples` table and its migration helpers, the examples-only validator, and the
store and server tests that cover them.

Deliverables:

- Delete `internal/store/example.go`, `internal/store/example_test.go`, and `internal/store/validate.go`.
- `internal/store/store.go`: remove `schema`, `exampleColumns`, `addMissingColumns`, `hasColumn`, `formatTime`, and
  `parseTime`; reduce `migrate` to a ping (or inline it into `New`); drop the now-unused `time` import; add
  `func (c *Client) Ping(ctx context.Context) error`.
- `internal/store/store_test.go`: drop `tableExists` and the examples-only tests
  (`TestNewAddsExamplesNSFWColumnToExistingDatabase`); rewrite `TestNewCreatesDatabaseAndSchema` to assert the database
  file exists and no tables are created; rewrite `TestNewReopensExistingDatabase` and `TestClosePreventsFurtherUse` to
  use `Ping` and a raw `SELECT` instead of examples; rewrite `TestNewAddsMissingTablesToExistingDatabase` to assert a
  pre-existing unrelated table and its row are left untouched.
- `internal/server/server_test.go`: `TestShutdownClosesStore` asserts `srv.store.Ping` fails after `Shutdown`.

Acceptance criteria:

- [x] A database created by `store.New` contains no `examples` table and no `reviews` table.
- [x] Opening a pre-existing database that still has a legacy table succeeds and does not modify or drop it.
- [x] `Ping` succeeds on an open store and fails after `Close`.
- [x] `internal/store` has no identifier referencing the examples feature.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/store/... ./internal/server/... -race -count=1` pass.

### Unit 4: documentation

Scope: the three docs that describe the feature or its setting.

Deliverables:

- `README.md`: delete the `EXAMPLES_ENABLED` bullet; drop "It tags the saved example and" from the `nsfw` sentence.
- `docs/PROMPT.md`: delete the "Model memory" section, whose only bullet was `examples`.
- `.env.example`: delete the "Saved examples (optional)" section and `EXAMPLES_ENABLED`. Keep `DB_PATH`.

Acceptance criteria:

- [x] No doc names the `examples` tool, `EXAMPLES_ENABLED`, or the `examples` table.
- [x] `DB_PATH` remains documented in `.env.example`.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all pass.
- A repo-wide search for the examples tool, setting, and store identifiers returns matches only in this plan and in
  `docs/plans/done`.
- Human, local: start the server against an existing `/data` database that has an `examples` table and confirm it boots.
  **Human.**

## Risks and follow-ups

- The store now has no users and creates no tables. It is kept only so the owner can see what still touches the database
  before deciding whether to delete it; a follow-up plan would remove `internal/store`, `DB_PATH`, and the lifecycle
  wiring in `internal/server`.
- The orphaned `examples` table keeps whatever prompts and image URLs were recorded. That is a privacy question, not a
  correctness one; deleting the table is a one-line manual `DROP TABLE examples` if wanted.
- `coverage.out` is an untracked local artifact and is now stale relative to the deleted files; it can be deleted or
  regenerated freely.
