# Self-hosted file serving

Status: Done
Depends on: none
Related: none.

## Goal

Store generated images on local disk and serve them over HTTP. Local storage is the only image backend: S3, Garagefront,
and URL shortening are removed by this plan, and so is the `publicize` tool and the per-call `public` flag. The service
always returns image URLs; it never returns base64 or inline image content, and there is no configuration that changes
that.

Served files are unguessable capability URLs. Every key contains a random UUID and access is anonymous: anyone with a
URL can read the file, and a file that is not present is simply not found. A reverse proxy or CDN in front of the
service can cache responses, because the URL for a given object never changes.

## Non-goals

- Authenticating served files beyond the unguessable URL.
- Signed URLs, server-side CDN behaviour, or smarter range handling than `http.ServeContent` provides.
- Serving non-image files.
- Indexing stored files in a database: the directory tree is the only index. The existing SQLite database stays for
  saved examples, which is unrelated to how files are addressed.

## Design

### URL contract and `PUBLIC_HOST`

`PUBLIC_HOST` is the absolute base URL clients use to reach this service, for example `https://neo.example.com` or
`http://192.168.1.10:8080`. It is the single source of truth for the scheme and host of every returned link, which is
what makes the "right URL type" come out for local, LAN, and reverse-proxied deployments. It is required, because
returned links must be absolute.

`PUBLIC_HOST` is validated as an absolute `http`/`https` URL with a non-empty host and no path component. A trailing
slash is accepted and trimmed. A base with a path, such as `https://example.com/neo`, is rejected: the routes are
mounted at the root, and the resolver maps `i/...` from the start of the path, so a prefix could not round-trip. A
reverse proxy that needs a prefix must rewrite it.

Local file URLs:

- `<PUBLIC_HOST>/i/<YYYY-MM>/<uuid>.<ext>`
- NSFW images get an `nsfw/` segment before the month directory:
  `<PUBLIC_HOST>/i/nsfw/<YYYY-MM>/<uuid>.<ext>`

### Web surface

The public web surface is served by the same process and the same `http.Server` as MCP, on the existing `HOST:PORT`.
`/mcp` stays behind the API key; `/i/*` is mounted without auth next to `/healthz`. This keeps one port to publish and
one volume to mount, and host-based separation stays available through a reverse proxy.

Every failure on the file routes is a `404`: a missing file, a file deleted after its URL was handed out, a traversal
attempt, a disallowed namespace, an extension outside the whitelist, and an unreadable file. Nothing on these routes
ever returns `401` or `403`, and no response leaks a filesystem path. Only whitelisted objects are ever returned: the
`i` namespace, and the extensions `png`, `jpg`, `jpeg`, `webp`, and `jxl`.

Responses carry `Cache-Control: public, max-age=31536000, immutable` and `X-Content-Type-Options: nosniff`, so a
reverse proxy or CDN can cache them safely.

### Storage layout

`FILES_DIR` holds the object tree; the key is the file path under it. Uploads write to a temporary file in the
destination directory and `rename` into place so a reader never sees a partial file. Extension comes from the output
media type through one shared mapping in `imgfmt`.

### Configuration

| Envar                  | Default | Meaning                                                                 |
| ---------------------- | ------- | ----------------------------------------------------------------------- |
| `PUBLIC_HOST`          | unset   | Absolute base URL for returned links. Required.                         |
| `FILES_DIR`            | `files` | Directory holding the object tree. Resolves to `/data/files` in Docker. |
| `FILES_RETENTION_DAYS` | `0`     | Delete stored files older than this many days. `0` keeps everything.    |

### No inline images

The MCP tools always upload generated images to the local store and return URLs. The inline base64 fallback in
`publishImages` is removed, so a storage failure surfaces as an error rather than a silent downgrade, and base64 image
content can never reach a client.

### Interfaces

`publish.Publisher` depends on an interface:

```go
type Store interface {
	UploadFile(ctx context.Context, data []byte, contentType string, nsfw bool) (string, error)
}
```

`resolve.ObjectStore` already fits the local store unchanged.

## Architecture impact

The mermaid diagram in `AGENTS.md` was updated:

- removed `s3upload` and `shortener`;
- added `filestore` to the backend and infrastructure clients block;
- added `server --> filestore` and `filestore --> imgfmt`;
- removed `publish --> s3upload` and `publish --> shortener`, since `publish` depends on its own `Store` interface.

## Implementation units

### Unit 1: remove the URL shortener

Deliverables:

- Deleted `internal/shortener/`.
- `internal/config/config.go`: removed `ShortenerAPIURL`, `ShortenerAPIKey`, `ShortenerExpiry`.
- `internal/publish/publish.go`: dropped the shortener field; `New` takes the uploader and a logger.
- `internal/mcp/server.go`, `internal/server/server.go`: removed shortener wiring.
- `.env.example`, `README.md`: removed the shortener sections.
- Deleted `docs/plans/SELF_HOSTED_URL_SHORTENER_PLAN.md`.

Acceptance criteria:

- [x] `SHORTENER_*` and "shortener" appear nowhere in the codebase.
- [x] `internal/shortener` no longer exists and nothing imports a shortener.
- [x] Generated URLs are returned exactly as the uploader produced them.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 2: remove S3 and Garagefront storage

Deliverables:

- Deleted `internal/s3upload/`.
- `internal/config/config.go`: removed every `S3_*` and `GARAGEFRONT_*` field and `GaragefrontPrefix`.
- `internal/publish/publish.go`: added the `Store` interface and takes it instead of `*s3upload.Client`.
- `internal/server/server.go`: removed S3 client construction and Garagefront handling.
- `go.mod`, `go.sum`: dropped the AWS SDK modules with `go mod tidy`.
- `.env.example`, `README.md`: removed the S3 and Garagefront sections.

Acceptance criteria:

- [x] `S3_*` and `GARAGEFRONT_*` appear nowhere in the codebase.
- [x] `internal/s3upload` no longer exists and no AWS SDK module is required.
- [x] `publish` depends only on its own `Store` interface.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 3: remove publicizing

Scope: everything stored is reachable by whoever can reach the service, so publishing is no longer a concept.

Deliverables:

- Deleted `internal/mcp/publicize.go` and its tests, and its registration.
- Removed `publishInput` and the per-call `public` flag from `txt2img`, `img2img`, and `bgkill`, including schema
  defaults and debug logs.
- `internal/mcp/publish.go`: removed the inline-image branch; `publishImages` always uploads.
- Updated `docs/PROMPT.md` to drop the `public` and `publicize` guidance.

Acceptance criteria:

- [x] No tool exposes a `public` flag and no `publicize` tool is registered.
- [x] `publishImages` has no inline fallback.
- [x] `go test ./... -race -count=1` passes.

### Unit 4: `PUBLIC_HOST` and file configuration

Deliverables:

- `internal/config/config.go`: added `PublicHost` (`PUBLIC_HOST`), `FilesDir` (`FILES_DIR`, default `files`), and
  `FilesRetentionDays` (`FILES_RETENTION_DAYS`, default `0`).
- `Config.PublicBase() (*url.URL, error)`: returns `nil, nil` when `PUBLIC_HOST` is unset; otherwise parses the
  trimmed value and errors when the scheme is not `http`/`https`, when the host is empty, or when a path is present.
- `internal/config/config_test.go`.

Acceptance criteria:

- [x] Every new envar has a default and an override test.
- [x] `PublicBase()` returns `nil, nil` when `PUBLIC_HOST` is empty and never returns a `*url.URL` with an empty host.
- [x] `PublicBase()` trims a trailing slash and rejects a value with a path component.
- [x] `go test ./... -race -count=1` passes.

### Unit 5: shared media type to extension mapping

Deliverables:

- `internal/imgfmt/format.go`: added `ExtensionForMediaType`, mapping `image/png` to `png`, `image/jpeg` to `jpg`,
  `image/jxl` to `jxl`, `image/webp` to `webp`, and anything else to `png`.
- `internal/imgfmt/format_test.go`.

Acceptance criteria:

- [x] Each supported media type maps to the extension used in keys.
- [x] An unknown media type falls back to `png`.

### Unit 6: local object store

Deliverables:

- `internal/filestore/store.go`:
    - `type Config struct { Dir string; PublicBase *url.URL; RetentionDays int }`;
    - `New(cfg Config, log *zap.Logger) (*Client, error)` creating `Dir` with mode `0o750` when missing, and erroring
      when `Dir` is empty or `PublicBase` is nil or has no host;
    - `UploadFile(ctx, data, contentType, nsfw) (string, error)` writing atomically and returning
      `PublicBase + "/" + key`;
    - `GetObject(ctx, key) ([]byte, error)`;
    - `objectKey(nsfw, ext)` producing `i/<YYYY-MM>/<uuid>.<ext>`, with an `nsfw` segment when requested;
    - `safePath(key)` rejecting empty keys, absolute paths, any `..` segment, keys outside the `i` namespace, and paths
      that resolve outside `Dir`.
- `internal/filestore/store_test.go`.

Acceptance criteria:

- [x] `UploadFile` then `GetObject` round-trips the exact bytes.
- [x] A plain upload returns a URL under `<PUBLIC_HOST>/i/`; an NSFW upload adds the `nsfw/` segment.
- [x] Keys are unique across identical uploads and carry the extension of the supplied media type.
- [x] `safePath` rejects `../../etc/passwd`, `/etc/passwd`, `x/y`, and `i/../../x`; it accepts a normal key.
- [x] `New` creates a missing directory, and `UploadFile` returns an error, not a panic, when the object tree cannot be
      created.
- [x] Tests use `t.TempDir()` and leave no files behind.

### Unit 7: serving stored files

Deliverables:

- `internal/filestore/handler.go`: `Register(r chi.Router)` mounting `GET` and `HEAD` for `/i/*`.
    - Every rejection is a `404` with no body detail.
    - Content type is set from an explicit extension whitelist; nothing is served with a sniffed type.
    - Sets `Cache-Control` and `X-Content-Type-Options`.
    - Streams with `http.ServeContent`.
- `internal/filestore/handler_test.go`.

Acceptance criteria:

- [x] `GET /i/<key>` returns the stored bytes and the correct `Content-Type`.
- [x] `HEAD` returns the same headers with an empty body and the correct length.
- [x] A `Range` request returns `206` with the requested slice.
- [x] A missing key, a traversal key, a disallowed namespace, and an unknown extension all return `404`.
- [x] A deleted file that was served before returns `404` on the next request.
- [x] No response body or header leaks a filesystem path, and no file route returns `401`, `403`, or `500`.
- [x] `Cache-Control` and `X-Content-Type-Options` are present on success.
- [x] A path outside `FILES_DIR` is never served, including when percent-encoded.

### Unit 8: local storage is the only backend

Deliverables:

- `internal/server/server.go`: requires `PUBLIC_HOST` and a non-empty `FILES_DIR`; builds `filestore`, registers its
  routes, logs `local files enabled`, and warns that stored files are readable by anyone with the URL; passes the store
  and `PUBLIC_HOST` to `resolve.New`.
- `internal/mcp/publish.go`: always uploads and returns URLs.
- `internal/mcp/publish_test.go`, `examples_test.go`, `provider_test.go`, `format_test.go`, `server_test.go`,
  `shared_test.go`: updated to the store-based publisher and the URL-only output.
- `internal/resolve/resolver.go`, `resolver_test.go`, `input_test.go`: reworded the S3/Garagefront comments and test
  names, and restricted the served namespace to `i`.
- `.env.example`, `README.md`, `.gitignore`.

Acceptance criteria:

- [x] A stored file is returned as a `<PUBLIC_HOST>/i/...` URL and the mounted route serves it; verified at the router
      level in `server_test.go`.
- [x] `examples` registers without any external storage configuration.
- [x] Missing or invalid `PUBLIC_HOST` and empty `FILES_DIR` each fail `server.New` naming the offending envar.
- [x] No code path returns base64 image content; `publishImages` always uploads.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 9: retention sweep and maintenance seam

Deliverables:

- `internal/filestore/retention.go`: `Sweep(now time.Time) (int, error)` walking the `i` tree, deleting files older than
  `FILES_RETENTION_DAYS`, then removing empty directories; a no-op when retention is `0`.
- `internal/server/server.go`: `RunMaintenance(ctx context.Context)` running the sweep every 6 hours until `ctx` is
  cancelled.
- `cmd/server/main.go`: runs `RunMaintenance` on a goroutine tied to the signal context.
- `internal/filestore/retention_test.go`, plus a `RunMaintenance` test in `server_test.go`.

Acceptance criteria:

- [x] `Sweep` deletes files older than the threshold and keeps newer ones and unrelated directories.
- [x] `Sweep` is a no-op when retention is `0`.
- [x] A swept file is no longer served.
- [x] `RunMaintenance` returns promptly when its context is cancelled.
- [x] `go test ./... -race -count=1` passes.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.
- Human, local: run with `FILES_DIR=/tmp/neo-files`, `PUBLIC_HOST=http://127.0.0.1:8080`, then call `txt2img`, open
  the returned URL, and confirm the image loads and that the same URL works from another device on the LAN when
  `PUBLIC_HOST` is set to a reachable address.
- Human, Docker: bind-mount `/data`, restart the container, and confirm previously returned URLs still resolve.

## Risks and follow-ups

- Local URLs are readable by anyone who has them. The startup warning and README wording are the whole mitigation.
- `FILES_DIR` on the same volume as the database means a full disk breaks writes and the store together.
- Serving and MCP share a listener; if MCP must never be reachable from where files are, a dedicated public listener is
  the follow-up, and it does not change `PUBLIC_HOST`.
- `EXAMPLES_MAX` prunes database rows but not files; retention is the only bound on disk use.
