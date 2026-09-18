# Self-hosted URL shortener

Status: Not started
Depends on: `SELF_HOSTED_FILE_SERVING_PLAN.md`, specifically `PUBLIC_HOST` and `Config.PublicBase()` (Unit 1) and the
route mounting and `RunMaintenance` seams (Units 5 and 6). The maintenance and resolver changes below extend those.
Related: none.

## Goal

Let the service host its own URL shortener, backed by the existing SQLite database and served from the public web
surface, as a fallback to `chhoto-url`. Short links are created for image URLs exactly as external ones are today, and
the `shorten` tool keeps working for arbitrary URLs.

## Non-goals

- Click analytics, per-link statistics, an admin UI, or custom or vanity slugs.
- Multiple short domains, or a shortener reachable on a different host from the image server.
- Changing `chhoto-url` behaviour; it remains the preferred backend when configured.

## Design

### Configuration

| Envar                      | Default | Meaning                                                                                      |
| -------------------------- | ------- | -------------------------------------------------------------------------------------------- |
| `SHORTENER_ENABLED`        | `false` | Use the built-in shortener. Mutually exclusive with `SHORTENER_API_URL`/`SHORTENER_API_KEY`. |
| `SHORTENER_SLUG_LENGTH`    | `7`     | Length of generated slugs, accepted in the range 4 to 32.                                    |
| `SHORTENER_EXPIRY_SECONDS` | `0`     | Existing envar, reused as the built-in link TTL. `0` never expires.                          |

`SHORTENER_ENABLED=true` requires `PUBLIC_HOST`, because the short URL is `<PUBLIC_HOST>/s/<slug>`, and the route is
mounted on the public web surface. Setting it together with `SHORTENER_API_URL` or `SHORTENER_API_KEY` fails startup
rather than silently picking one, so a stale chhoto configuration cannot mask the switch.

Backend selection for the shortener mirrors storage:

1. `SHORTENER_API_URL` and `SHORTENER_API_KEY` both set: the existing `chhoto-url` client.
2. Otherwise `SHORTENER_ENABLED`: the built-in local shortener.
3. Otherwise no shortener, and the `shorten` tool is not registered.

### Link shape

A short link is `<PUBLIC_HOST>/s/<slug>` where the slug is random base62 from `crypto/rand`. Random slugs are used
rather than a counter so links cannot be enumerated, and because the slug is the only protection on the destination.
An eight character default would be overkill for this service's expected volume; seven is kept configurable.

Redirect responses use `302 Found` because a link can expire and because browsers must revalidate on each use.

### Expiry

`SHORTENER_EXPIRY_SECONDS` sets `expires_at = created_at + TTL`; `0` stores no expiry. Lookup compares against the
current time: an unknown slug is `404`, an expired slug is `410 Gone`, and the expired row is deleted on the way out
so it is not re-read. `PruneShortLinks(ctx, now)` is exposed for the maintenance tick so untouched expired rows do not
accumulate forever.

### Resolution

`resolve.Resolver` currently sends any URL on the public base that is not under `i`/`a` to an HTTP fetch. A local short
link would therefore need the container to reach its own `PUBLIC_HOST`, which fails on many NAT and DNS setups even
though the redirect target is local. The resolver gains an optional link lookup for `/s/<slug>` paths on its public
base, so `publicize` and `bgkill` can accept a local short URL without a network hop. A slug that is not found still
falls through to the HTTP path, which keeps `chhoto-url` and external shorteners working.

### Interfaces

`publish.Publisher` and `internal/mcp` hold a concrete `*shortener.Client`. They narrow to an interface so the local
implementation is interchangeable:

```go
type Shortener interface {
	Shorten(ctx context.Context, url string) (string, error)
}
```

`*shortener.Client` and `*shortener.Local` both satisfy it. Constructor wiring must pass an untyped `nil` when no
shortener is configured, so `== nil` checks keep working.

## Architecture impact

Update the mermaid diagram in `AGENTS.md`:

- add the `shortener --> store` edge for the built-in backend;
- keep `resolve --> shortener` absent: the lookup interface is declared in `internal/resolve`, and the local shortener
  satisfies it structurally.

## Implementation units

### Unit 1: short link persistence

Scope: schema and queries in the existing store package.

Deliverables:

- `internal/store/shortlink.go`:
    - `short_links` table (`id`, `slug` unique, `url`, `created_at`, `expires_at` nullable) added to the schema list, so
      existing databases gain it through the existing `migrate` path with no manual step;
    - `MaxSlugLength` alongside the other length constants, since callers configure a shorter length than this;
    - `type ShortLink struct { ID int64; Slug, URL string; CreatedAt time.Time; ExpiresAt *time.Time }`;
    - `SaveShortLink(ctx, link ShortLink) error`, returning a sentinel `ErrSlugTaken` on a unique violation so callers
      can retry;
    - `ShortLink(ctx, slug string) (ShortLink, error)`, returning `ErrShortLinkNotFound`;
    - `DeleteShortLink(ctx, slug string) error`;
    - `PruneShortLinks(ctx, now time.Time) (int, error)`.
- `internal/store/shortlink_test.go`.

Acceptance criteria:

- [ ] Validation rejects an empty slug, a slug outside `[0-9A-Za-z]`, a slug longer than `MaxSlugLength`, a
      non-absolute URL, and a non-http(s) URL.
- [ ] Saving a duplicate slug returns `ErrSlugTaken` and leaves the first row intact.
- [ ] `ShortLink` returns `ErrShortLinkNotFound` for an unknown slug.
- [ ] `PruneShortLinks` deletes only rows whose `expires_at` is before `now` and leaves never-expiring rows alone.
- [ ] A database created before this change gains the table on open.

### Unit 2: built-in shortener service

Scope: slug generation and the `Shortener` implementation plus the interface.

Deliverables:

- `internal/shortener/shorten.go`: add the `Shortener` interface.
- `internal/shortener/local.go`, with this shape:

    ```go
    type LocalConfig struct {
    	Store      *store.Client
    	Base       *url.URL
    	SlugLength int
    	TTL        time.Duration
    	Rand       io.Reader // defaults to crypto/rand.Reader
    }

    func NewLocal(cfg LocalConfig, log *zap.Logger) (*Local, error)
    ```

    `NewLocal` validates the slug length, a non-nil base, and a non-nil store, and defaults `Rand` so tests can inject a
    deterministic source. The type provides:
    - `Shorten(ctx, longURL string) (string, error)`: validate the URL is absolute http(s), generate a slug from the
      random source, retry on `ErrSlugTaken` up to a small bounded number of attempts, persist with the TTL, and return
      `<base>/s/<slug>`;
    - `Destination(ctx, slug string) (string, error)` returning `ErrShortLinkNotFound` or `ErrShortLinkExpired`
      sentinels, deleting the row when expired;
    - `Lookup(ctx, slug string) (string, bool, error)` returning the destination and `true` for a live link,
      `("", false, nil)` for an unknown or expired one, and a wrapped error on storage failure; this is the adapter
      `resolve` consumes, so `resolve` needs no import of this package;
    - `Prune(ctx, now time.Time) (int, error)` delegating to the store.

- `internal/shortener/local_test.go`, using a temporary database from `store.New` and an injectable random source so
  collisions and slug content are deterministic in tests.

Acceptance criteria:

- [ ] A shortened URL is absolute, starts with the configured base, and uses the `/s/` prefix.
- [ ] Slugs match `[0-9A-Za-z]{SHORTENER_SLUG_LENGTH}` and differ across calls.
- [ ] A forced collision is retried and ultimately succeeds; exhausting attempts returns an error rather than looping.
- [ ] A relative or non-http input is rejected before anything is written.
- [ ] With a TTL set, `Destination` returns the destination before expiry and an expiry error after it, and deletes the
      row.
- [ ] With no TTL, `Destination` never expires.
- [ ] `*Client` (chhoto) and `*Local` both satisfy `Shortener`, and `*Local` satisfies the resolver's lookup
      interface without either package importing the other.

### Unit 3: redirect handler

Scope: serve short links on the public web surface.

Deliverables:

- `internal/shortener/handler.go`: `Register(r chi.Router)` mounting `GET` and `HEAD` for `/s/{slug}`.
    - Valid slug and a live link: `302` with a `Location` header and no body.
    - Unknown slug: `404`. Expired slug: `410`.
    - Invalid slug characters: `404`, without a database lookup.
    - `Cache-Control: no-store` on all responses, so an expired link is never cached as a redirect.
- `internal/shortener/handler_test.go`.

Acceptance criteria:

- [ ] A live link redirects to its exact stored URL with `302` and the expected `Location`.
- [ ] Unknown, expired, and malformed slugs return `404`, `410`, and `404` respectively.
- [ ] Every response carries `Cache-Control: no-store` and leaks no internal detail.
- [ ] `HEAD` behaves like `GET` minus the body.
- [ ] No redirect is issued for a slug whose stored URL has since become invalid.

### Unit 4: wiring, resolution, and documentation

Scope: config, backend selection, resolver integration, and the tool.

Deliverables:

- `internal/config/config.go`: add `ShortenerEnabled` (`SHORTENER_ENABLED`, default `false`) and `ShortenerSlugLength`
  (`SHORTENER_SLUG_LENGTH`, default `7`), reusing the existing `ShortenerExpiry`.
- `internal/server/server.go`:
    - error when `SHORTENER_ENABLED` is set without a valid `PUBLIC_HOST`;
    - error when `SHORTENER_ENABLED` is set alongside `SHORTENER_API_URL` or `SHORTENER_API_KEY`, naming both envars;
    - error when `SHORTENER_SLUG_LENGTH` is outside 4 to 32;
    - build `shortener.Local`, register its routes, and log `local shortener enabled`;
    - extend `RunMaintenance` to prune expired links on the same tick when the local shortener is active;
    - pass the shortener to `resolve` through `WithLinkStore` when its public base matches `PUBLIC_HOST`.
- `internal/resolve/resolver.go`: add `WithLinkStore(lookup LinkStore) *Resolver`, where

    ```go
    type LinkStore interface {
    	Lookup(ctx context.Context, slug string) (string, bool, error)
    }
    ```

    The resolver consults it only when the request host matches its public base and the path is `/s/<slug>`: a hit
    replaces the URL under resolution and the loop continues, a miss falls through to the existing HTTP fetch.

- `internal/publish/publish.go`, `internal/mcp/server.go`, `internal/mcp/handlers.go`, `internal/mcp/shorten.go`:
  switch to the `shortener.Shortener` interface; registration continues to be driven by a non-nil shortener.
- `internal/server/server_test.go`, `internal/config/config_test.go`, `internal/resolve/resolver_test.go`: coverage for
  the validation, selection, and lookup behaviour.
- `.env.example` and `README.md`: document the new envars and the local fallback.

Acceptance criteria:

- [ ] `SHORTENER_ENABLED=true` alone (no chhoto config) registers `shorten`, and shortening a URL returns
      `<PUBLIC_HOST>/s/<slug>` that the mounted route resolves.
- [ ] `SHORTENER_ENABLED=true` together with `SHORTENER_API_URL` or `SHORTENER_API_KEY` fails startup naming both.
- [ ] `SHORTENER_ENABLED=true` without `PUBLIC_HOST` fails startup naming `PUBLIC_HOST`.
- [ ] `SHORTENER_SLUG_LENGTH` of `3` and of `33` fail; `7` succeeds.
- [ ] `chhoto-url` configuration alone is unchanged, and neither shortener configured leaves `shorten` unregistered.
- [ ] `publicize` given a local short URL resolves it without an outbound HTTP request (asserted with an HTTP client
      that fails the test if called) when the destination is a local file.
- [ ] Image URLs produced by `txt2img` are shortened when the local shortener is active.
- [ ] `RunMaintenance` prunes expired links and still returns on cancellation.
- [ ] `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` pass.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`.
- Human, local: enable local files and the built-in shortener, call `txt2img`, and confirm the returned short link
  redirects to a working image from a browser on another device.
- Human: set `SHORTENER_EXPIRY_SECONDS=60`, create a link, and confirm it redirects before the TTL and returns `410`
  afterwards.

## Risks and follow-ups

- Slugs are the only protection on the destination; a short link to a private image is effectively public.
- Random slugs rely on `crypto/rand`; a failing entropy source must surface as an error, not a weak slug.
- A single short domain is assumed. Multiple domains would need a domain column and per-request host selection.
- Expired rows are pruned on the maintenance tick and lazily on access; a deployment with the shortener on but
  maintenance off would accumulate rows, so maintenance is enabled whenever the shortener is.
