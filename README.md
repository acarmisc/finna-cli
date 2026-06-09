# finna

Go CLI for the [finna-app](https://github.com/acarmisc/finna-app) FinOps backend.

> Status: **early development** — phases 0–2 of the implementation plan are landed (bootstrap, config/contexts, API client). Auth, resource commands, and dashboard are upcoming.

## Install

```sh
# coming soon (Homebrew tap)
brew tap acarmisc/finna
brew install finna
```

For now, build from source (see Development below).

## Quickstart

```sh
finna context add prod --server https://finna.example.com
finna context use prod
finna version
```

Run `finna` with no args to see the command tree.

### Configuration

Config file lives at `$XDG_CONFIG_HOME/finna/config.toml` (fallback `~/.config/finna/config.toml`). Tokens are stored in the OS keyring, never on disk.

Settings can be overridden in this order: CLI flag → environment variable → config file → default.

Env vars: `FINNA_SERVER`, `FINNA_CONTEXT`, `FINNA_OUTPUT`, `FINNA_TOKEN`, `FINNA_DEBUG`, `NO_COLOR`.

## Development

Requirements: Go 1.24+ (oapi-codegen needs it), `golangci-lint` v2.

```sh
make build      # produces ./finna
make test       # unit tests
make lint       # golangci-lint
make generate   # regenerate API client from api/openapi.yaml
make tidy       # go mod tidy
```

Install `golangci-lint`:

```sh
brew install golangci-lint
# or: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
```

### Regenerating the API client

The OpenAPI spec is vendored at `api/openapi.yaml`. To refresh:

```sh
cp ../finna-app/docs/openapi.yaml api/openapi.yaml
make generate
```

## License

MIT — see [LICENSE](./LICENSE).
