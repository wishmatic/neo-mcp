# Neo MCP

A thin MCP wrapper exposing image capabilities to LLM agents. Main functionality leans on SD WebUI Forge.

## Features

Note that the only version of Forge we support is
[Haoming02's fork](https://github.com/Haoming02/sd-webui-forge-classic/tree/neo#stable-diffusion-webui-forge---neo).

- Core feature: `txt2img` and `img2img` tools are implemented with full control of knobs to adjust image generation.
    - They also route to NovelAI when `model` starts with `nai-diffusion-`. Set `NOVELAI_API_KEY` to enable it. Forge-only
      options such as hi-res fix, presets, and VAE/text encoders are ignored for NovelAI requests.
    - With NovelAI enabled, `anlas` reports the account's credit balance and the V5 usage meter.
- `bgkill` removes the background from an image via the
  [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet) extension. It can optionally crop to the
  foreground and produce a padded square, which is handy for logo generation.
- `img2txt` recognises an image and returns a textual response via any OpenAI-compatible vision endpoint. Set
  `IMG2TXT_BASE_URL`, `IMG2TXT_API_KEY`, and `IMG2TXT_MODEL` to enable it; `IMG2TXT_SYSTEM_PROMPT` optionally supplies a
  default system instruction. The image may be a URL, a base64 data URI, or raw base64 PNG, JPEG, or WebP data.
- If you provide `S3_*` environment variables, generated images are uploaded to an S3-compatible object store and
  returned as presigned URLs.
    - Without this, they're returned as plain `image/png` content.
    - If you also provide `GARAGEFRONT_URL` and `GARAGEFRONT_USER_ID`, images are uploaded under that user's
      directory and returned as unsigned URLs served by [Garagefront](https://github.com/wishmatic/garagefront).
      Super niche.
- If you _also_ provide `SHORTENER_*` environment variables, any generated URLs are shortened first.
    - This assumes your URL shortener is [`chhoto-url`](https://github.com/SinTan1729/chhoto-url).

## Usage

Deploy as a Docker image:

```sh
docker run -d \
  -p 8080:8080 \
  -e API_KEY=change-me \
  -e SD_URL=http://host.docker.internal:7860 \
  ghcr.io/wishmatic/neo-mcp:latest
```

The MCP endpoint is served at `/mcp`.

All other configuration is optional but strongly recommended; see [.env.example](.env.example).

### SD Web UI Forge Neo's API

You need to turn on the API for SD Web UI Forge Neo for this to work. Add `--api` to your Forge launch command, then
point `SD_URL` at your Forge instance's API.

No support will be provided for any issues related to your installation of Forge, nor the Forge API.

### Authentication

`API_KEY` is required on every request, sent as `Authorization: Bearer <API_KEY>`.

### Agent Model Knowledge

Your agent will need knowledge of VAE and text encoder models as well as available upscalers in order to use them. Add
these verbatim to your system prompt or a skill, along with checkpoints, LoRA, and anything else it needs. The same goes
for NovelAI model ids: no list is maintained here, so the agent supplies them.

### S3 and URL Shortening

Please see [.env.example](.env.example) for a configuration.

### Garagefront

Garagefront support is single user for now: `GARAGEFRONT_USER_ID` is static, so every image is uploaded to one
LibreChat user's directory. Please raise an issue if you want multi-user, especially if you'd want it without an admin
having to add an envar per user.

### Extensions Support

Some extensions I use will be supported over time if they can be called via the Forge API.

Right now, this is only [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet), which provides the
`bgkill` tool.

## Warning

This MCP will be available and maintained so long as I use it, and is built for my own purposes. Extending features via
issue requests and PRs will be _considered_ but unless I find use out of it myself, I probably won't work on those
features.

This is also very bespoke to my use case. I recommend forking this and adjusting features to your needs if it doesn't
quite fit your own.

## License

Neo MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by AUTOMATIC1111, Illyasviel, Haoming02, Amazon Web Services
(particularly S3), `chhoto-url`, or any other third-party services. Those names are trademarks of their respective
owners and are used here only to describe compatibility.
