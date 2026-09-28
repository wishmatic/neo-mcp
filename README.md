# Neo MCP

<img src="docs/images/logo.webp" alt="Neo MCP Logo" width="128">

A thin MCP wrapper exposing image capabilities to LLM agents.

## Features

Note that the only version of Forge we support is
[Haoming02's fork](https://github.com/Haoming02/sd-webui-forge-classic/tree/neo#stable-diffusion-webui-forge---neo).

- Core feature: `txt2img` and `img2img` to generate images.
  - They also route to NovelAI when `model` starts with `nai-diffusion-`.
    - Set `NOVELAI_API_KEY` to enable this.
  - Forge-only ptions such as hi-res fix, presets, and VAE/text encoders are ignored for NovelAI.
- With NovelAI enabled, `anlas` reports the account's credit balance.
- Not just PNG; WebP output by default, and JPEG and JXL are supported too!
  - Adjust default output with `OUTPUT_FORMAT`.
- Every call to an image tool stores the image, returns its URL as text, and attaches the image as
  an MCP image block such that a vision-capable agent can see the returned result.
  - This also means that this isn't just an MCP; it also is a simple image hosting service!
- `bgkill` removes the background from an image via the
  [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet) extension for Forge.
- `crop` trims an image to the bounding box of its visible content, optionally padding, centering
  or even circularising it! Great for logos and avatars. This runs locally and does not require
  Forge nor NovelAI.
- `convert` re-encodes an image as `png`, `jpeg`, `jxl`, or `webp`. Also runs locally!
- Private networking support; input URLs can be mapped before they are fetched, even to a local
  directory seen by the container.

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

The MCP endpoint is served at `/mcp`.

`/data` holds the image store, so bind-mount a host directory there to keep files across container
replacements; the container runs as uid 65532, so that directory must be writable by it.

All other configuration is optional but strongly recommended; see [.env.example](.env.example).

### SD Web UI Forge Neo's API

You need to turn on the API for SD Web UI Forge Neo for this to work. Add `--api` to your Forge
launch command, then point `SD_URL` at your Forge instance's API.

No support will be provided for any issues related to your installation of Forge, nor the Forge API.

### Authentication

`API_KEY` is required on every `/mcp` request, sent as `Authorization: Bearer <API_KEY>`. Stored
images are served without authentication.

### Agent Model Knowledge

Your agent will need knowledge of VAE and text encoder models as well as available upscalers in
order to use them. Add these verbatim to your system prompt or a skill, along with checkpoints,
LoRA, and anything else it needs. The same goes for NovelAI model ids: no list is maintained here,
so the agent supplies them.

### Stored Images

Please see [.env.example](.env.example) for a configuration. Put `PUBLIC_HOST` behind a reverse
proxy or CDN if you want caching; responses are immutable and cacheable.

### Extensions Support

Some extensions I use will be supported over time if they can be called via the Forge API.

Right now, this is only [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet),
which provides the `bgkill` tool.

## Warnings

This MCP will be available and maintained so long as I use it, and is built for my own purposes.
Extending features via issue requests and PRs will be _considered_ but unless I find use out of it
myself, I probably won't work on those features.

This is also very bespoke to my use case. I recommend forking this and adjusting features to your
needs if it doesn't quite fit your own.

## License

Neo MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by AUTOMATIC1111, Illyasviel, Haoming02, or any
other third-party services. Those names are trademarks of their respective owners and are used here
only to describe compatibility.
