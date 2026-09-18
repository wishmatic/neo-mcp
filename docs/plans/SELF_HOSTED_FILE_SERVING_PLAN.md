# Self-hosted file serving

Status: Not started
Depends on: none
Related: `SELF_HOSTED_URL_SHORTENER_PLAN.md` depends on Unit 1 and the route mounting and maintenance seams
introduced here.

## Goal

Store generated images on local disk and serve them over HTTP, so the service can return usable image URLs without S3
or Garagefront. S3 stays the recommended backend; this is a fallback for deployments that do not have one.

## Non-goals

- Replacing S3, Garagefront, or any existing S3 behaviour.
- Authenticating served files. Local URLs are unguessable capability URLs: anyone holding one can read it.
- Signed URLs, CDN behaviour, or smarter range handling than `http.ServeContent` provides.
- Serving non-image files.

## Design

### URL contract and `PUBLIC_HOST`

New envar `PUBLIC_HOST` is the absolute base URL clients use to reach this service, for example
`https://neo.example.com` or `http://192.168.1.10:8080`. It is the single source of truth for the scheme, host, and
optional path prefix of every returned link, which is what makes the "right URL type" come out for local, LAN, and
reverse-proxied deployments. It is required whenever the local file server or the built-in shortener is enabled,
because returned links must be absolute.

`PUBLIC_HOST` is validated as an absolute `http`/`https` URL with a non-empty host and no path component. A trailing
slash is accepted and trimmed. A base with a path, such as `https://example.com/neo`, is rejected: the routes are
mounted at the root, and the resolver maps `i/...` from the start of the path, so a prefix could not round-trip. A
reverse proxy that needs a prefix must rewrite it.

Local file URLs deliberately reuse Garagefront's shape so that downstream behaviour (resolver key mapping, the
`public` flag, shortening, and examples) is unchanged:

- public namespace: `<PUBLIC_HOST>/i/public/<YYYY-MM>/<uuid>.<ext>`
- private namespace: `<PUBLIC_HOST>/i/images/<YYYY-MM>/<uuid>.<ext>`

### Web surface

The public web surface is served by the same process and the same `http.Server` as MCP, on the existing `HOST:PORT`.
`/mcp` stays behind the API key; `/i/*` (and later `/s/*`) is mounted without auth next to `/healthz`. This keeps one
port to publish and one volume to mount, and host-based separation stays available through a reverse proxy. A dedicated
listener can be added later without changing the `PUBLIC_HOST` contract, and is not needed for this fallback.

### Storage layout

`FILES_DIR` holds the object tree; the key is the file path under it. Uploads write to a temporary file in the
destination directory and `rename` into place so a reader never sees a partial file. Extension comes from the output
media type, using one shared mapping so the S3 and local backends cannot disagree.

### Configuration

| Envar                  | Default | Meaning                                                                                              |
| ---------------------- | ------- | ---------------------------------------------------------------------------------------------------- |
| `PUBLIC_HOST`          | unset   | Absolute base URL for returned links. Required when `FILES_ENABLED` or the built-in shortener is on. |
| `FILES_ENABLED`        | `false` | Use local disk for image storage. Ignored, with a warning, when S3 is configured.                    |
| `FILES_DIR`            | `files` | Directory holding the object tree. Resolves to `/data/files` in Docker.                              |
| `FILES_RETENTION_DAYS` | `0`     | Delete stored files older than this many days. `0` keeps everything.                                 |

Precedence for the storage backend is fixed and validated at startup:

1. S3 configured (`S3_*` complete) wins; if `FILES_ENABLED` is also set, log a warning that it is ignored.
2. Otherwise `FILES_ENABLED` uses the local store.
3. Otherwise images stay inline, as today.

The resolver's public base follows the same choice: `GARAGEFRONT_URL` when S3 is used, `PUBLIC_HOST` when local storage
is used. Local URLs then resolve to bytes by object key through the existing `resolve` path instead of an HTTP round
trip.

### Interfaces

`publish.Publisher` currently depends on the concrete `*s3upload.Client`. It narrows to an interface so either backend
can be injected:

```go
type Store interface {
	UploadFile(ctx context.Context, data []byte, contentType string, public bool) (string, error)
}
```

`resolve.ObjectStore` already fits the local store unchanged.

## Architecture impact

Update the mermaid diagram in `AGENTS.md`:

- add `filestore` to the backend and infrastructure clients block;
- add `server --> filestore`;
- add `filestore --> imgfmt` and `s3upload --> imgfmt` (shared media type to extension mapping);
- remove `publish --> s3upload`, since `publish` depends on its own `Store` interface instead of the concrete client.

## Implementation units

Each unit is intended to be reviewable as its own change. `internal/server`, `internal/config`, and `internal/publish`
changes here overlap with `SELF_HOSTED_URL_SHORTENER_PLAN.md`, so implement this plan first.

### Unit 1: `PUBLIC_HOST` and file configuration

Scope: config surface and validation only, no behaviour.

Deliverables:

- `internal/config/config.go`: add `PublicHost` (`PUBLIC_HOST`), `FilesEnabled` (`FILES_ENABLED`, default `false`),
  `FilesDir` (`FILES_DIR`, default `files`), `FilesRetentionDays` (`FILES_RETENTION_DAYS`, default `0`).
- `Config.PublicBase() (*url.URL, error)`: returns `nil, nil` when `PUBLIC_HOST` is unset; otherwise parses the
  trimmed value and errors when the scheme is not `http`/`https`, when the host is empty, or when a path is present.
- `internal/config/config_test.go`: defaults for each new envar; override for each; `PublicBase` cases for unset,
  valid, valid with trailing slash trimmed, missing scheme, non-http scheme, empty host, and a path component.

Acceptance criteria:

- [ ] Every new envar has a default and an override test.
- [ ] `PublicBase()` returns `nil, nil` when `PUBLIC_HOST` is empty and never returns a `*url.URL` with an empty host.
- [ ] `PublicBase()` trims a trailing slash and rejects a value with a path component.
- [ ] `go test ./... -race -count=1` passes.

### Unit 2: shared media type to extension mapping

Scope: remove the duplicated mapping before a second backend can drift from it.

Deliverables:

- `internal/imgfmt/format.go`: add `ExtensionForMediaType(mediaType string) string`, mapping `image/png` to `png`,
  `image/jpeg` to `jpg`, `image/jxl` to `jxl`, `image/webp` to `webp`, and anything else to `png` (matching the
  current S3 default).
- `internal/imgfmt/format_test.go`: a case per known media type plus an unknown one.
- `internal/s3upload/upload.go`: delete `fileExtension` and call `imgfmt.ExtensionForMediaType`.

Acceptance criteria:

- [ ] No content type to extension switch remains outside `internal/imgfmt`.
- [ ] Existing `s3upload` upload tests pass unchanged.

### Unit 3: local object store

Scope: the storage half of `internal/filestore`, no HTTP yet.

Deliverables:

- `internal/filestore/store.go`:
    - `type Config struct { Dir string; PublicBase *url.URL }`;
    - `New(cfg Config, log *zap.Logger) (*Client, error)` creating `Dir` with mode `0o750` when missing, and erroring
      when `Dir` is empty or `PublicBase` is nil or has no host;
    - `UploadFile(ctx context.Context, data []byte, contentType string, public bool) (string, error)` writing atomically
      and returning `PublicBase + "/" + key`;
    - `GetObject(ctx, key string) ([]byte, error)`;
    - `objectKey(public bool, ext string) string` producing `i/public/<YYYY-MM>/<uuid>.<ext>` or
      `i/images/<YYYY-MM>/<uuid>.<ext>`;
    - `safePath(key string) (string, error)` rejecting empty keys, absolute paths, any `..` segment, keys outside the
      `i`/`a` namespaces, and paths that resolve outside `Dir` (`filepath.Rel` based check).
- `internal/filestore/store_test.go`.

Acceptance criteria:

- [ ] `UploadFile` then `GetObject` round-trips the exact bytes.
- [ ] A public upload returns a URL under `<PUBLIC_HOST>/i/public/` and a private upload under
      `<PUBLIC_HOST>/i/images/`.
- [ ] Keys are unique across identical uploads and carry the extension of the supplied media type.
- [ ] `safePath` rejects `../../etc/passwd`, `/etc/passwd`, `x/y`, and `i/../../x`; it accepts a normal key.
- [ ] `New` creates a missing directory, and `UploadFile` returns an error, not a panic, when the directory is not
      writable.
- [ ] Tests use `t.TempDir()` and leave no files behind.

### Unit 4: serving stored files

Scope: the HTTP half of `internal/filestore`.

Deliverables:

- `internal/filestore/handler.go`: `Register(r chi.Router)` mounting `GET` and `HEAD` for `/i/*` and `/a/*`.
    - Requests map to `safePath`; anything rejected is a `404`.
    - Content type is set from an explicit extension map (`png`, `jpg`/`jpeg`, `webp`, `jxl`); an unknown extension is a
      `404`, so nothing is served with a sniffed type.
    - Sets `Cache-Control: public, max-age=31536000, immutable` and `X-Content-Type-Options: nosniff`.
    - Streams with `http.ServeContent`, which supplies `HEAD`, `Range`, and conditional requests.
- `internal/filestore/handler_test.go`.

Acceptance criteria:

- [ ] `GET /i/public/<key>` returns the stored bytes and the correct `Content-Type`.
- [ ] `HEAD` returns the same headers with an empty body and the correct length.
- [ ] A `Range` request returns `206` with the requested slice.
- [ ] A missing key, a traversal key, and an unknown extension all return `404` and leak no filesystem detail.
- [ ] A path outside `FILES_DIR` is never served, including when percent-encoded.
- [ ] `Cache-Control` and `X-Content-Type-Options` are present on success.

### Unit 5: backend selection and wiring

Scope: make the service choose and use exactly one backend.

Deliverables:

- `internal/publish/publish.go`: add the `Store` interface; store it in `Publisher`; `New` takes a `Store`. Confirm
  `Enabled()` is false for a nil interface and true for either concrete backend.
- `internal/server/server.go`:
    - validate `PUBLIC_HOST` via `PublicBase()` and return an error naming `PUBLIC_HOST` when `FILES_ENABLED` is set but
      it is missing or invalid;
    - build the S3 uploader first; when S3 is configured and `FILES_ENABLED` is set, log a warning that local storage is
      ignored;
    - otherwise build `filestore` when `FILES_ENABLED` is set, register its routes, and log `local files enabled` with
      the directory and retention;
    - log a warning that stored files are readable by anyone with the URL;
    - pass `GARAGEFRONT_URL` or `PUBLIC_HOST` to `resolve.New` according to the chosen backend.
- `internal/mcp/publicize.go`: change the "S3 upload is not configured" message to name image storage generally, and
  update `TestPublicizeWithoutUploader` to match.
- `internal/server/server_test.go`: backend selection, validation, and route reachability via `httptest`.
- `.env.example`: add the four envars with a commented example.
- `README.md`: a short bullet in Features and a sentence in the S3 section saying local storage is the fallback.
- `.gitignore`: add `files/`.

Acceptance criteria:

- [ ] With no S3 and `FILES_ENABLED=true`, `txt2img` returns `<PUBLIC_HOST>/i/...` URLs and the mounted route serves
      them; verified at the router level in `server_test.go`.
- [ ] `publicize` and `examples` register when local storage is the only backend.
- [ ] `FILES_ENABLED=true` without `PUBLIC_HOST`, with an invalid `PUBLIC_HOST`, and with an empty `FILES_DIR` each
      fail `server.New` with an error naming the offending envar.
- [ ] With both S3 and `FILES_ENABLED` configured, S3 is used and exactly one warning is logged.
- [ ] Logs contain no misleading success message when no backend is configured.
- [ ] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 6: retention sweep and maintenance seam

Scope: bound disk growth, and give later periodic work a lifecycle to join.

Deliverables:

- `internal/filestore/retention.go`: `Sweep(now time.Time) (int, error)` walking `Dir`, deleting files whose
  modification time is older than `FILES_RETENTION_DAYS`, then removing empty month directories; a no-op when
  retention is `0`.
- `internal/server/server.go`: `RunMaintenance(ctx context.Context)` running the sweep on a fixed interval (every 6
  hours) until `ctx` is cancelled, logging each run and its deletions. It blocks until cancellation, and returns
  immediately when there is nothing configured to maintain.
- `cmd/server/main.go`: run `RunMaintenance` on a goroutine tied to the signal context.
- Tests: `retention_test.go` calling `Sweep` directly with a fixed `now`, plus a `RunMaintenance` test asserting it
  returns on cancellation.

Acceptance criteria:

- [ ] `Sweep` deletes files older than the threshold and keeps newer ones and unrelated directories.
- [ ] `Sweep` is a no-op when retention is `0`.
- [ ] A swept file is no longer served.
- [ ] `RunMaintenance` returns promptly when its context is cancelled and does not leak a goroutine
      (`goleak`-style check is not required; assert prompt return).
- [ ] `go test ./... -race -count=1` passes.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`.
- Human, local: run with `FILES_ENABLED=true`, `FILES_DIR=/tmp/neo-files`, `PUBLIC_HOST=http://127.0.0.1:8080`, then
  call `txt2img`, open the returned URL, and confirm the image loads and that the same URL works from another device
  on the LAN when `PUBLIC_HOST` is set to a reachable address.
- Human, Docker: bind-mount `/data`, restart the container, and confirm previously returned URLs still resolve.

## Risks and follow-ups

- Local URLs are readable by anyone who has them. The startup warning and README wording are the whole mitigation.
- `FILES_DIR` on the same volume as the database means a full disk breaks writes and the store together.
- Serving and MCP share a listener; if MCP must never be reachable from where files are, a dedicated public listener is
  the follow-up, and it does not change `PUBLIC_HOST`.
- `EXAMPLES_MAX` prunes database rows but not files; retention is the only bound on disk use.
