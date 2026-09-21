# Image URL Map Plan

Status: Done
Depends on: none
Copied from: `internal/sourcemap` and `docs/plans/done/INLINE_URL_SOURCES_PLAN.md` in Elfu MCP, `../elfu-mcp`

## Goal

Let Neo MCP reach the image URLs an agent actually has, by mapping a public URL prefix to somewhere this service can
read it from, for any number of prefixes, configured in one envar:

```sh
IMAGE_URL_MAP=https://example.com=http://example:5080,https://chat.example.com/images/=/data/librechat-data
```

An `img2img` or `bgkill` call whose URL falls under a public key has the key's private side substituted before the
fetch: `https://example.com/i/x.webp` is fetched from `http://example:5080/i/x.webp`, and a URL under a directory key is
read from disk instead of over HTTP. A URL that matches no key is fetched exactly as it is today.

## What is being copied

Elfu MCP already solved this, and Neo takes its implementation as it is. The copy is literal:

- `internal/sourcemap` is copied from Elfu's package with the same names, signatures, semantics, and error style. That
  package imports nothing outside the standard library and nothing from its own module, so it is copied verbatim, tests
  included, and needs no edits at all.
- The same envar grammar: comma-separated `public=private` pairs, where the private side is an absolute `http(s)` base
  URL or an absolute directory.
- The same matching: scheme and host equal, host case-insensitively, path prefix at a boundary, longest matching path
  wins.
- The same placement inside `resolve.Resolver.Fetch`: per hop, immediately after the stored-object short-circuit, reading
  from disk for a directory entry and rewriting in place for a base URL entry.
- The same observability rule: the private side of an entry is never logged.

The one intentional difference is the envar name, `IMAGE_URL_MAP` rather than Elfu's `INLINE_URL_MAP`, because Neo has no
`inline` tool: both of its URL inputs are image URLs.

Nothing here calls Elfu, reads Elfu's configuration, or shares a map with it. The copy is standalone, and each service
configures its own map.

## Motivation

Both URL inputs Neo accepts hit the cases this map exists for:

- `init_image_url` and `image_url` are handed whatever URL the user has, including one that cannot be downloaded without
  a session cookie or bearer token, while the same bytes sit on a volume Neo can have mounted. LibreChat's uploaded
  images are the motivating one: the agent asks the user for the URL the chat shows, and Neo reads the file from the
  mounted directory.
- A URL on a public host that does not resolve inside Neo's network, but does resolve under a private name, such as an
  image served by another service on that network.

## Scope

In scope:

- The `IMAGE_URL_MAP` envar: comma-separated `public=private` pairs, parsed and validated at startup, failing fast with
  an error that names `IMAGE_URL_MAP`.
- A new leaf package, `internal/sourcemap`, copied from Elfu's: keys, both kinds of private side, longest-prefix
  matching, URL rewriting, and directory reads.
- Applying the map inside `internal/resolve`, per hop, so direct fetches and every redirect hop are covered.
- Documentation: `README.md`, `docs/PROMPT.md`, `.env.example`, and the architecture diagram and leaf rule in `AGENTS.md`.

Out of scope:

- Calling Elfu or sharing a map with it. Each service configures its own.
- Wildcards, regular expressions, or host-suffix matching. Keys are literal, and a host-wide key is a key with no path.
- Rewriting the URLs Neo returns. `PUBLIC_HOST` behaviour is unchanged, and a `PUBLIC_HOST` URL still reads from the
  local file store.
- Rewriting outbound calls to Forge or NovelAI. `SD_URL` and the NovelAI base URL already point at internal addresses.
- Changing tool descriptions or input schemas. A caller never needs to know that a URL was mapped.
- Preserving the original `Host` header. Rewriting the URL means the private host is what Go sends as `Host`, which suits
  the intended deployments (a service name or IP where the far side does not vhost on the public name). The copy behaves
  exactly as Elfu's does.
- Refreshing the map at runtime. It is fixed for the process lifetime.

## Design

### One envar, two kinds of private side

`IMAGE_URL_MAP` is a plain string, parsed by `sourcemap.Parse`, holding comma-separated `public=private` pairs:

```sh
# a path prefix on a public host, read from disk, as LibreChat's images directory mounted into this container
IMAGE_URL_MAP=https://chat.example.com/images/=/data/librechat-data

# a public host, fetched from a private base URL
IMAGE_URL_MAP=https://example.com=http://example:5080
```

- Whitespace around a pair and around both sides of the `=` is trimmed. The split is on the first `=`, so a URL
  containing one is fine on either side.
- Blank entries are skipped, so `IMAGE_URL_MAP=` and a trailing comma mean "no extra pairs" rather than an error, which
  matters because a compose or shell interpolation of an unset variable passes an empty string. An unset, empty, or
  all-blank value means every URL is fetched as it is today.
- A key is an absolute `http` or `https` URL prefix, optionally with a path, and with no query or fragment. Keys are
  unique. A value is either an absolute `http`/`https` base URL, or an absolute directory.
- A malformed pair is a startup error, not a silent skip, since a typo would otherwise surface much later as a fetch
  failure.

### Matching

A URL matches a key when its scheme and host are equal, case-insensitively, and its path starts with the key's path at a
boundary. A path match must be a real boundary: `https://h/img` does not match `https://h/images/x`. The longest matching
path wins, so a specific prefix overrides a host-wide key for the URLs under it. Matching is scheme-sensitive, so
`https://example.com` and `http://example.com` are different keys, and a port is part of the host as written.

### What a match does

- A base URL entry appends the remainder of the path, and the original query, to the private base:
  `https://example.com/images/a.png?x=1` under `https://example.com/images/=http://librechat:3080/images` becomes
  `http://librechat:3080/images/a.png?x=1`. The rewritten URL is what the request is built from.
- A directory entry reads the file instead. The remainder is URL-decoded, cleaned, and joined under the directory. The
  read is refused when the remainder is empty, when it is absolute, when the joined path escapes the directory (checked
  against it the same way the file store checks a key against its own directory), when the extension is not one Neo
  decodes, or when the file exceeds the 32 MiB cap. A refusal is an error, the same as a failed fetch, so the tool
  reports it.
- The extension whitelist is `.png`, `.jpg`, `.jpeg`, `.webp`, and `.jxl`, matching `imgfmt`'s formats. Together with the
  escape check, it is what keeps a mapped directory from becoming an arbitrary local file read.

### Where it is applied

`internal/resolve` is the only component that turns a caller-supplied URL into an outbound request, so it is the only
place that changes, and it changes the way Elfu's does:

- `resolve.New` gains a third parameter, `sources *sourcemap.Map` (nil and empty both mean "fetch as today"). The one
  production call site in `internal/server`, and the test call sites in `internal/resolve` and `internal/mcp`, are
  updated for the extra argument.
- Inside `Resolver.Fetch`'s existing per-hop loop, immediately after the stored-object short-circuit: a directory match
  returns the file bytes; a base URL match replaces the hop URL with the rewritten target and the loop continues. The
  stored-object check runs first on the unrewritten URL, so a URL on `PUBLIC_HOST` still reads from the local file store
  even when its host is also mapped.
- Because the rewrite happens per hop, a redirect into a mapped prefix is rewritten on that hop too, whether it came from
  an unmapped or a mapped URL. A relative redirect is resolved against the URL of the hop that returned it.
- Errors and non-200 messages name the URL of the hop that failed, which after a rewrite is the rewritten URL. Handlers
  already log the caller's `init_image_url`/`image_url` next to the error, so the original stays traceable.

### The secret that is not a secret

A directory entry hands Neo the ability to read a directory, and nothing authenticates the caller beyond the URL they
supply, so the key being long and unguessable is the operator's whole protection. Two consequences, the same ones Elfu
documents:

- Neo logs the URL it was asked to read, at debug for every call and on failure. A key that is secret can therefore
  appear in this service's logs and in the conversation it came from. It is unguessable, not confidential.
- A base URL entry needs no such property: it is a routing convenience, not a capability.
- The private side of an entry, whether a directory or an internal base URL, is never logged. Nothing about the map is
  logged at startup.

### Config and wiring

- `config.Config` gains `ImageURLMap string` with `env:"IMAGE_URL_MAP"`, carried raw the way Elfu carries
  `INLINE_URL_MAP`, since `sourcemap.Parse` takes the whole spec as a string.
- `internal/server` parses it with `sourcemap.Parse` next to the existing `PublicBase()` handling, wraps any error as
  `IMAGE_URL_MAP`, and passes the map into `resolve.New`.
- `resolve` gains one dependency, `internal/sourcemap`, where it has none today.

### Package layout

`internal/sourcemap` is a leaf: standard library only (`errors`, `fmt`, `net/url`, `os`, `path/filepath`, `strings`), no
imports of other packages in this module, in the spirit of `internal/utils`.

```mermaid
flowchart LR
    server["internal/server"] --> sourcemap["internal/sourcemap"]
    resolve["internal/resolve"] --> sourcemap
```

`AGENTS.md` gains those two arrows, and its architecture prose gains `internal/sourcemap` beside `internal/utils` as a
shared leaf that must stay dependency-free.

## Implementation units

### Unit 1: copy `internal/sourcemap`

Deliverables: `internal/sourcemap/sourcemap.go` and `internal/sourcemap/sourcemap_test.go`, copied from
`../elfu-mcp/internal/sourcemap` without edits.

Acceptance criteria:

- [x] Both files are byte-identical to Elfu's. The package is standard library only, so the copy needs no changes, and
      the diff against `../elfu-mcp/internal/sourcemap` is empty.
- [x] Elfu's tests pass unchanged in Neo, covering, per their names: an empty spec matching nothing, rejected entries
      (no separator, empty private side, empty public side, missing scheme, wrong scheme, relative directory, query in
      the private side, duplicate key), both kinds in one spec, longest path winning, match boundaries and case, the
      rewritten target for a host-wide key, a path key, a preserved query, and an empty remainder, `FilePath` refusals
      and a nested image path, and `Read` returning unescaped file contents, refusing a missing file, a directory, an
      oversized file, and an escape, plus whitespace trimming.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass, and `go list -deps
./internal/sourcemap` shows no package of this module.

### Unit 2: resolving through the map

Deliverables: `internal/resolve/resolver.go`, `internal/resolve/resolver_test.go`, `internal/resolve/input_test.go`,
`internal/config/config.go`, `internal/config/config_test.go`, `internal/server/server.go`, and the `resolve.New` call
sites in `internal/mcp`'s tests.

Acceptance criteria:

- [x] `resolve.New` takes the map, and a nil or empty map leaves today's behaviour unchanged, with the existing resolver
      tests passing after the extra argument only.
- [x] A URL under a base URL entry is fetched from the rewritten base, proven by an `httptest` server that stands in for
      the private host and receives the request with the path and query preserved.
- [x] A URL under a directory entry is read from disk and returned, and no HTTP request is made (assert zero requests on
      the test server).
- [x] A URL under a directory entry aiming outside the directory, at an unknown extension, or at a file over the cap
      returns an error rather than bytes.
- [x] A mapped prefix reached by a redirect is rewritten on that hop, from both a mapped and an unmapped start URL.
- [x] A URL on `PUBLIC_HOST` whose path is `i/...` still reads from the object store when its host is also a key, with no
      request to the test server, so the stored-object short-circuit keeps precedence.
- [x] `IMAGE_URL_MAP` parses into config, and a malformed value makes `server.New` fail at startup with an error naming
      `IMAGE_URL_MAP`.
- [x] Nothing about the map is logged at startup.
- [x] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

### Unit 3: documentation

Deliverables: `README.md`, `docs/PROMPT.md`, `.env.example`, `AGENTS.md`.

Acceptance criteria:

- [x] `README.md` documents `IMAGE_URL_MAP` with an example of each kind, states that a URL under a key is rewritten
      before it is fetched, and states that a directory entry's key is its only access control, so it must be long.
- [x] `docs/PROMPT.md` tells the agent it can pass the URL the user pasted straight to the image tools, since the server
      maps URLs it cannot reach, mirroring Elfu's `docs/PROMPT.md`.
- [x] `.env.example` lists `IMAGE_URL_MAP` with a commented example of each kind, appended at the end of the file
      because the file is excluded from agent reads.
- [x] `AGENTS.md`'s diagram adds `internal/sourcemap` with arrows from `internal/server` and `internal/resolve`, and its
      leaf rule names `internal/sourcemap` beside `internal/utils`, checked against `go list`.

## Verification

Per unit, `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1`, as CI runs them, plus `go list -deps`
for the package boundary. The map, the rewrite, the directory reads, and the escape checks are all testable with a temp
directory and an `httptest` server, so the parts that matter are covered without a deployment.

Human checks, marked as such because they need real hosts and mounts:

- [ ] Set `IMAGE_URL_MAP=https://<public-host>=http://<private-host>:<port>` where the private host serves the same
      bytes, call `bgkill` with `image_url=https://<public-host>/i/<key>`, and confirm the image comes back and the
      private host's access log shows the request while the public host sees none. Repeat for `img2img` with
      `init_image_url`.
- [ ] Mount a real LibreChat images directory, paste an image, have the agent pass the URL the chat showed, and confirm
      `img2img` uses it as the init image.
