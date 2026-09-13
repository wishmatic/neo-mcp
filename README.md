# Neo MCP

A thin MCP wrapper around a local Stable Diffusion WebUI (Forge Neo) instance.

## Features

Note that the only version of Forge we support is
[Haoming02's fork](https://github.com/Haoming02/sd-webui-forge-classic/tree/neo#stable-diffusion-webui-forge---neo).

- Core feature: `txt2img` and `img2img` tools are implemented with full control of knobs to adjust image generation.
- If you provide `S3_*` environment variables, generated images are uploaded to an S3-compatible object store and
  returned as presigned URLs.
    - Without this, they're returned as plain `image/png` content.
- If you _also_ provide `SHORTENER_*` environment variables, any generated URLs are shortened first.
    - This assumes your URL shortener is [`chhoto-url`](https://github.com/SinTan1729/chhoto-url).

## Usage

Deploy as a Docker image. Instructions pending.

### SD Web UI Forge Neo's API

You need to turn on the API for SD Web UI Forge Neo for this to work. Add `--api` to your Forge launch command, then
point `SD_URL` at your Forge instance's API.

No support will be provided for any issues related to your installation of Forge, nor the Forge API.

### Authentication

`API_KEY` is required on every request, sent as `Authorization: Bearer <API_KEY>`.

### Agent Model Knowledge

Your agent will need knowledge of VAE and text encoder models as well as available upscalers in order to use them. Add
these verbatim to your system prompt or a skill, along with checkpoints, LoRA, and anything else it needs.

### S3 and URL Shortening

Please see [.env.example](.env.example) for a configuration.

### Extensions Support

Some extensions I use will be supported over time if they can be called via the Forge API.

Right now, this is only [`sd-webui-birefnet`](https://github.com/dimitribarbot/sd-webui-birefnet) for removing
backgrounds. [TODO: This isn't done yet.]

## Warning

This MCP will be available and maintained so long as I use it, and is built for my own purposes. Extending features via
issue requests and PRs will be _considered_ but unless I find use out of it myself, I probably won't work on those
features.

This is also very bespoke to my use case. I recommend forking this and removing features to your needs if it doesn't
quite fit your own.

## License

Neo MCP is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

This project is not affiliated with or endorsed by AUTOMATIC1111, Illyasviel, Haoming02, Amazon Web Services
(particularly S3), `chhoto-url`, or any other third-party services. Those names are trademarks of their respective
owners and are used here only to describe compatibility.
