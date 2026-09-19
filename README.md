# Neo MCP

A thin MCP wrapper exposing image capabilities to LLM agents. Main functionality leans on SD WebUI Forge.

## Features

Note that the only version of Forge we support is
[Haoming02's fork](https://github.com/Haoming02/sd-webui-forge-classic/tree/neo#stable-diffusion-webui-forge---neo).

- Core feature: `txt2img` and `img2img` tools are implemented with full control of knobs to adjust image generation.
    - They also route to NovelAI when `model` starts with `nai-diffusion-`. Set `NOVELAI_API_KEY` to enable it. Forge-only
      options such as hi-res fix, presets, and VAE/text encoders are ignored for NovelAI requests.
    - With NovelAI enabled, `anlas` reports the account's credit balance and the V5 usage meter.
    - Both take an optional `nsfw` boolean to mark a generation as NSFW. It tags the saved example and places the image
      under an `nsfw/` subdirectory; it does not change generation.
- Output images are WebP by default. `OUTPUT_FORMAT` sets the format for every tool (`png`, `jpeg`, `jxl`, or `webp`),
  and the `format` input overrides it for a single call. Transparency is kept for every format except JPEG, which
  composites onto white.
- `bgkill` removes the background from an image via the
  [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet) extension. It can optionally crop to the
  foreground and produce a padded square, which is handy for logo generation.
- Generated images are written to local disk and returned as URLs served by this service. Set `PUBLIC_HOST` to the base
  URL clients use to reach it, and `FILES_DIR` for where files live (`/data/files` in Docker).
    - Stored files are unguessable and anonymous: anyone holding a URL can open the image, and nobody else can. There
      is no other access control, and images are never returned inline, so every tool call produces a URL.
    - Mount `FILES_DIR` on a volume to keep files across container replacements.
    - `FILES_RETENTION_DAYS` deletes files older than that many days; `0` keeps everything.
- If you set `EXAMPLES_ENABLED=true`, every `txt2img`/`img2img` query and the URL of the image it produced are saved per
  model, and the `examples` tool returns random ones: two by default, or as many as you ask for with `n`. A generation's
  `nsfw` flag tags its example, and the call's `nsfw` filters them: `-1` non-NSFW only, `1` NSFW only, `0` or omitted for
  either. Only the newest `EXAMPLES_MAX` examples per model are kept.
    - The database is created automatically at `DB_PATH` (`neo-mcp.db`; `/data/neo-mcp.db` in Docker).

## Usage

Deploy as a Docker image:

```sh
docker run -d \
  -p 8080:8080 \
  -e API_KEY=change-me \
  -e PUBLIC_HOST=http://192.168.1.10:8080 \
  -e SD_URL=http://host.docker.internal:7860 \
  -v /mnt/user/appdata/neo-mcp:/data \
  ghcr.io/wishmatic/neo-mcp:latest
```

The MCP endpoint is served at `/mcp`; stored images are served from `/i/`.

`/data` holds the SQLite database and, by default, the image store, so bind-mount a host directory there to keep them
across container replacements; the container runs as uid 65532, so that directory must be writable by it.

All other configuration is optional but strongly recommended; see [.env.example](.env.example).

### SD Web UI Forge Neo's API

You need to turn on the API for SD Web UI Forge Neo for this to work. Add `--api` to your Forge launch command, then
point `SD_URL` at your Forge instance's API.

No support will be provided for any issues related to your installation of Forge, nor the Forge API.

### Authentication

`API_KEY` is required on every `/mcp` request, sent as `Authorization: Bearer <API_KEY>`. Stored images are served
without authentication.

### Agent Model Knowledge

Your agent will need knowledge of VAE and text encoder models as well as available upscalers in order to use them. Add
these verbatim to your system prompt or a skill, along with checkpoints, LoRA, and anything else it needs. The same goes
for NovelAI model ids: no list is maintained here, so the agent supplies them.

### Stored Images

Please see [.env.example](.env.example) for a configuration. Put `PUBLIC_HOST` behind a reverse proxy or CDN if you want
caching; responses are immutable and cacheable.

### Extensions Support

Some extensions I use will be supported over time if they can be called via the Forge API.

Right now, this is only [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet), which provides the
`bgkill` tool.

## Warnings

This MCP will be available and maintained so long as I use it, and is built for my own purposes. Extending features via
issue requests and PRs will be _considered_ but unless I find use out of it myself, I probably won't work on those
features.

This is also very bespoke to my use case. I recommend forking this and adjusting features to your needs if it doesn't
quite fit your own.

Current human understanding of this codebase is: 50%.

## License

Neo MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by AUTOMATIC1111, Illyasviel, Haoming02, or any other third-party
services. Those names are trademarks of their respective owners and are used here only to describe compatibility.
