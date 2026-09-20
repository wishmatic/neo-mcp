# Remove file retention days

Status: Done. All units implemented; `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` are green. One owner-run check remains, listed under Verification.
Depends on: `docs/plans/done/SELF_HOSTED_FILE_SERVING_PLAN.md` (which introduced retention).
Related: `internal/filestore/retention.go`, `internal/server/server.go`, `internal/config/config.go`, `cmd/server/main.go`.

## Goal

Remove the concept of file retention days. `FILES_RETENTION_DAYS` is gone, and stored files are always retained. The
periodic maintenance sweep that existed only to expire files is removed with it.

## Non-goals

- Deleting any files that a previous retention setting already expired.
- Changing how files are stored, named, served, or how `FILES_DIR` works.

## Design

| Unit | Layer          | Files                                                                                        |
| ---- | -------------- | -------------------------------------------------------------------------------------------- |
| 1    | Server and cmd | `internal/server/{server.go,server_test.go}`, `cmd/server/main.go`                           |
| 2    | Filestore      | `internal/filestore/{retention.go,retention_test.go,store.go,store_test.go,handler_test.go}` |
| 3    | Configuration  | `internal/config/{config.go,config_test.go}`                                                 |
| 4    | Documentation  | `.env.example`, `README.md`                                                                  |

Server-side first: Unit 1 removes the only callers of `Client.Sweep` and the only readers of `filestore.Config.RetentionDays`,
so the filestore code Unit 2 deletes is already unreferenced. Unit 3 then drops the `Config` field that Unit 1 stopped
reading.

### What stays

- `internal/filestore`: `Client`, `New`, `UploadFile`, `GetObject`, `safePath`, `objectKey`, `writeFileAtomic`, and
  `Register`/`serve`. Files are written and served exactly as before.
- `Server.Run`, `Server.Shutdown`, and the `"local files enabled"` log line (minus the `retention_days` field).
- `context` and `time` imports in `internal/server`, still used by `Run`, `Shutdown`, and the HTTP timeouts.

### What goes

- `internal/filestore/retention.go` and `retention_test.go`: `Sweep`, `deleteOlderThan`, `removeEmptyDirs`, and the
  `backdate` test helper.
- `filestore.Config.RetentionDays`.
- `internal/server/server.go`: the `maintenanceInterval` constant, `RunMaintenance`, `sweep`,
  `RetentionDays: cfg.FilesRetentionDays`, and the `retention_days` log field.
- `cmd/server/main.go`: the `go srv.RunMaintenance(ctx)` goroutine.
- `internal/config/config.go`: the `FilesRetentionDays` field and `FILES_RETENTION_DAYS`.

## Recorded decisions

- `TestRunMaintenanceReturnsOnCancel` is deleted: it tested the sweep loop, which no longer exists.
- `newTestClient` in `internal/filestore/store_test.go` loses its `retentionDays` parameter; all call sites (in
  `store_test.go` and `handler_test.go`) drop the argument.
- `TestLoadFilesDefaults` and `TestLoadFiles` keep covering `PUBLIC_HOST` and `FILES_DIR`; only their retention
  assertions and env handling are removed.

## Implementation units

### Unit 1: server and cmd maintenance removal

Scope: the periodic sweep loop and its wiring.

Deliverables:

- `internal/server/server.go`: reduce the `const` block to `writeTimeout`; drop `RetentionDays: cfg.FilesRetentionDays`
  from the `filestore.Config` literal; drop the `retention_days` field from the `"local files enabled"` log call; delete
  `RunMaintenance` and `sweep`.
- `cmd/server/main.go`: delete `go srv.RunMaintenance(ctx)`.
- `internal/server/server_test.go`: delete `TestRunMaintenanceReturnsOnCancel` and the now-unused `time` import.

Acceptance criteria:

- [x] `internal/server` has no `RunMaintenance`, `sweep`, `maintenanceInterval`, or retention reference.
- [x] The server boots and serves stored files with retention removed.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/server/... -race -count=1` pass.

### Unit 2: filestore retention removal

Scope: the sweep implementation and the `Config` field behind it.

Deliverables:

- Delete `internal/filestore/retention.go` and `internal/filestore/retention_test.go`.
- `internal/filestore/store.go`: remove `RetentionDays` from `Config`.
- `internal/filestore/store_test.go`: change `newTestClient` to `func newTestClient(t *testing.T) (*Client, string)` and
  update its call sites.
- `internal/filestore/handler_test.go`: update the `newTestClient` call in `newTestRouter`.

Acceptance criteria:

- [x] `internal/filestore` has no `Sweep`, `RetentionDays`, or retention reference.
- [x] Uploading, reading, and serving objects behave exactly as before.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/filestore/... -race -count=1` pass.

### Unit 3: configuration

Scope: the `FILES_RETENTION_DAYS` setting.

Deliverables:

- `internal/config/config.go`: remove the `FilesRetentionDays` field.
- `internal/config/config_test.go`: in `TestLoadFilesDefaults`, drop the `FILES_RETENTION_DAYS` unset and its assertion;
  in `TestLoadFiles`, drop the `FILES_RETENTION_DAYS` set and its assertion.

Acceptance criteria:

- [x] `internal/config` has no `FilesRetentionDays` or `FILES_RETENTION_DAYS` reference.
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/config/... -race -count=1` pass.

### Unit 4: documentation

Scope: the docs that describe the setting.

Deliverables:

- `.env.example`: delete the `0 retention days means no deletion.` comment and `FILES_RETENTION_DAYS=0`.
- `README.md`: delete the `FILES_RETENTION_DAYS` bullet.

Acceptance criteria:

- [x] Neither doc names `FILES_RETENTION_DAYS`.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all pass.
- A repo-wide search for `RetentionDays`, `FILES_RETENTION_DAYS`, `Sweep`, and `RunMaintenance` returns matches only in
  the done plans.
- Human, local: start the server, upload an image, and confirm it is served and never removed. **Human.**

## Risks and follow-ups

- Disk use is now unbounded. That is the intended behavior, but it means an owner who previously relied on retention
  must prune `FILES_DIR` themselves.
