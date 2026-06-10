# finna

Go CLI for the [finna-app](https://github.com/acarmisc/finna-app) FinOps backend.

> Status: **v0.1.0** — auth, configs, projects, extractors, runs, costs, dashboard, alerts, wastage, and diagnostics are all implemented.

## Install

### Homebrew (macOS / Linux)

```sh
brew tap acarmisc/finna
brew install finna-cli
```

### Direct download

Download the latest release archive and checksum from
[GitHub Releases](https://github.com/acarmisc/finna-cli/releases), then verify
and extract:

```sh
# example for macOS arm64
curl -LO https://github.com/acarmisc/finna-cli/releases/latest/download/finna_<VERSION>_darwin_arm64.tar.gz
curl -LO https://github.com/acarmisc/finna-cli/releases/latest/download/checksums.txt
sha256sum --check --ignore-missing checksums.txt
tar -xzf finna_<VERSION>_darwin_arm64.tar.gz
sudo mv finna /usr/local/bin/
```

### From source

```sh
go install github.com/acarmisc/finna-cli/cmd/finna@latest
```

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
make build        # produces ./finna (injects version/commit/date)
make test         # unit tests with race detector
make lint         # golangci-lint
make generate     # regenerate API client from api/openapi.yaml
make tidy         # go mod tidy
make snapshot     # goreleaser snapshot (all platforms, no publish)
make release-dry  # alias for snapshot
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
