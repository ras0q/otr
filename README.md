# OTR — Open Tool Registry 🦦

CLI tools from GitHub Releases, installed as npm packages. Use npm, pnpm, Yarn, or Bun as usual.

Public registry: `https://TBD/` (TBD).

## Setup

`.npmrc`:

```ini
@otr:registry=https://TBD/
```

`package.json` (`github.com/owner/repo` → `@otr/owner--repo`):

```json
{
  "devDependencies": {
    "@otr/cli--cli": "2.83.1",
    "@otr/astral-sh--uv": "0.6.11"
  }
}
```

## Run

```bash
pnpm install
pnpm exec gh auth status
pnpm exec uv --version
```

Optional `scripts`:

```json
{
  "scripts": {
    "release": "gh release create v1.0.0"
  }
}
```

## Self-host

Set `@scope:registry=` to your instance. Run `go run ./cmd/otr/main.go --help` for server options.
