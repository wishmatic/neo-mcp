# Remove the database

Status: Done. All units implemented; `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` are green. One owner-run check remains, listed under Verification.
Depends on: `docs/plans/done/REMOVE_EXAMPLES_PLAN.md`.
Related: `internal/store/`, `internal/server/server.go`, `internal/config/config.go`, `AGENTS.md`.

## Goal

Remove the SQLite database and everything wired to it: the `internal/store` package, the `DB_PATH` setting, the
`modernc.org/sqlite` dependency tree, the store lifecycle in `internal/server`, and every doc mention. After the
examples removal nothing reads or writes rows, so the database is pure overhead.

## Non-goals

- Dropping data from existing database files on disk. See Recorded decisions.
- Changing anything about local file storage (`internal/filestore`, `/data`, `FILES_DIR`), which is unaffected.

## Design

| Unit | Layer                 | Files                                                           |
| ---- | --------------------- | --------------------------------------------------------------- |
| 1    | Store and server      | `internal/store/`, `internal/server/{server.go,server_test.go}` |
| 2    | Configuration         | `internal/config/{config.go,config_test.go}`                    |
| 3    | Dependencies          | `go.mod`, `go.sum`                                              |
| 4    | Docs and architecture | `.env.example`, `README.md`, `AGENTS.md`                        |

The store is only imported by `internal/server`, so removing both together keeps the tree compiling. Unit 2 drops
`DB_PATH`, which Unit 1 no longer reads.

### What stays

- The image pipeline: `filestore`, `publish`, `resolve`, and `/data` as the image root.
- `Server.Shutdown`, which still shuts the HTTP server down; it just no longer closes a database.
- The `errors` import in `internal/server`, which `Run` still uses via `errors.Is`.

### What goes

- `internal/store/store.go` and `internal/store/store_test.go`: `Client`, `New`, `Close`, `Ping`, and `dsn`.
- `internal/server/server.go`: the `internal/store` import, the `Server.store` field, the `store.New(cfg.DBPath)` block
  and its `"database opened"` log line, the two `storeClient.Close()` error paths, `store: storeClient` in the returned
  `Server`, and `s.store.Close()` in `Shutdown`.
- `internal/config/config.go`: the `DBPath` field and `DB_PATH`.
- `go.mod`/`go.sum`: `modernc.org/sqlite` plus its indirect modules `modernc.org/libc`, `modernc.org/mathutil`,
  `modernc.org/memory`, `github.com/ncruces/go-strftime`, `github.com/dustin/go-humanize`,
  `github.com/remyoudompheng/bigfft`, and `github.com/mattn/go-isatty`.

## Recorded decisions

- No `DROP`/file-deletion runs against existing databases. A `neo-mcp.db` left in `/data` is harmless and can be
  removed by hand; silently deleting user files is worse than leaving an unused one.
- `TestNewCreatesDatabase`, `TestNewFailsOnUnopenableDatabase`, and `TestShutdownClosesStore` in
  `internal/server/server_test.go` are deleted, not rewritten: they tested database behavior that no longer exists.
  `TestShutdownClosesStore` is replaced by a `TestShutdown` that asserts `Shutdown` returns nil.
- `internal/config/config_test.go` loses `TestLoadDBPathDefaults` and `TestLoadDBPath`.
- The `AGENTS.md` architecture diagram loses the `store` node and its three inbound edges (`server`, `mcp`, `present`).

## Implementation units

### Unit 1: store and server

Scope: the store package and the `internal/server` lifecycle that opens and closes it.

Deliverables:

- Delete `internal/store/store.go` and `internal/store/store_test.go` (and the package directory).
- `internal/server/server.go`: remove the `internal/store` import and the `Server.store` field; delete the
  `storeClient, err := store.New(cfg.DBPath)` block, the `"database opened"` log line, both `_ = storeClient.Close()`
  error-path calls, the `store: storeClient` field in the returned `Server`, and the `s.store.Close()` term in
  `Shutdown` (leaving `return s.http.Shutdown(ctx)`).
- `internal/server/server_test.go`: drop `DBPath` from `testConfig`; delete `TestNewCreatesDatabase`,
  `TestNewFailsOnUnopenableDatabase`, and `TestShutdownClosesStore`; add `TestShutdown`; drop the `os.Stat(cfg.DBPath)`
  assertion and the now-unused `os` import.

Acceptance criteria:

- [x] `internal/server` has no identifier referencing a database or `internal/store`.
- [x] `New` no longer creates a database file for any input.
- [x] `Shutdown` returns the HTTP server's shutdown error and nothing else.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/server/... -race -count=1` pass.

### Unit 2: configuration

Scope: the `DB_PATH` setting.

Deliverables:

- `internal/config/config.go`: remove the `DBPath` field.
- `internal/config/config_test.go`: delete `TestLoadDBPathDefaults` and `TestLoadDBPath`.

Acceptance criteria:

- [x] `internal/config` has no `DBPath` or `DB_PATH` reference.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/config/... -race -count=1` pass.

### Unit 3: dependencies

Scope: the module graph.

Deliverables:

- Run `go mod tidy` after Units 1 and 2.

Acceptance criteria:

- [x] `go.mod` no longer requires `modernc.org/sqlite` or its indirect-only modules.
- [x] `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` pass.

### Unit 4: docs and architecture

Scope: the docs that mention the database plus the architecture diagram.

Deliverables:

- `.env.example`: delete the `Database (optional)` section and `DB_PATH`.
- `README.md`: reword the `/data` line so it names the image store only.
- `AGENTS.md`: remove the `store["internal/store"]` node and the `server --> store`, `mcp --> store`, and
  `present --> store` edges from the architecture diagram.

Acceptance criteria:

- [x] No doc names `DB_PATH` or an `internal/store` package.
- [x] The architecture diagram still lists every remaining package and edge.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all pass.
- `go mod why modernc.org/sqlite` reports that the module is no longer needed.
- A repo-wide search for `DBPath`, `DB_PATH`, `internal/store`, and `modernc.org/sqlite` returns matches only in the
  done plans.
- Human, local: start the server against a `/data` directory that still holds an old `neo-mcp.db` and confirm it boots
  and serves images. **Human.**

## Risks and follow-ups

- An old `neo-mcp.db` (and its `-wal`/`-shm` siblings) stays on disk where it was. Delete by hand if the recorded
  prompts and URLs are a privacy concern.
- `coverage.out` is an untracked local artifact and is now stale relative to the deleted files; it can be deleted or
  regenerated freely.
