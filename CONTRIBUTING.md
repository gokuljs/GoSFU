# Contributing

Thanks for being interested in contributing to GoSFU.

This project is meant to be forked, studied, and adapted. Contributions are welcome, including bug fixes, documentation improvements, provider integrations, deployment notes, and improvements to the voice-agent loop.

## Before Opening a PR

For new features, provider changes, transport changes, or orchestration changes, please open an issue first and describe the use case. This helps keep the approach clear before implementation starts.

Small fixes and documentation improvements can go straight to a pull request.

Please avoid low-effort drive-by PRs. A good PR should show that you understand the relevant part of the project and should be easier to review than to rewrite.

## Local Setup

Follow the setup instructions in the [README](README.md).

Do not commit `.env` files, API keys, local credentials, generated binaries, or large model artifacts.

## Repository Layout

- `sfu/` is the Go backend.
- `sfu-frontend/` is the React/Vite debug console.
- Provider implementations usually live under `sfu/plugins/`.

## Checks

Run the relevant checks for the area you changed.

Backend:

```bash
cd sfu
go test ./...
```

Frontend:

```bash
cd sfu-frontend
bun run lint
bun run typecheck
bun run build
```

If a check cannot run locally because of platform dependencies, missing provider credentials, or ONNX Runtime setup, mention that in the PR.

## Pull Requests

- Keep PRs focused.
- Explain what changed and why.
- Include the checks you ran.

