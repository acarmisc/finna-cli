# finna

[![CI](https://github.com/acarmisc/finna-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/acarmisc/finna-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/acarmisc/finna-cli)](https://github.com/acarmisc/finna-cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://go.dev/)

**CLI for [finna-app](https://github.com/acarmisc/finna-app)** — manage cloud cost extractors, monitor spend, and act on wastage findings from your terminal.

```
$ finna dashboard
 Extractors   4 registered · 1 running · 0 failed (24h)
 Costs (MTD)  GCP $1,243.80 ▲12%  Azure $892.10 ▼4%
 Alerts       2 active  ● warning  ● info
 Wastage      3 findings · $340/mo potential savings
```

---

## Install

### Homebrew (macOS / Linux)

```sh
brew tap acarmisc/finna
brew install finna-cli
```

### Direct download

Grab the latest binary from [GitHub Releases](https://github.com/acarmisc/finna-cli/releases):

```sh
# macOS arm64
curl -LO https://github.com/acarmisc/finna-cli/releases/latest/download/finna_Darwin_arm64.tar.gz
tar -xzf finna_Darwin_arm64.tar.gz
sudo mv finna /usr/local/bin/
```

### From source

```sh
go install github.com/acarmisc/finna-cli/cmd/finna@latest
```

---

## Quickstart

```sh
# 1. Point finna at your server
finna context add prod --server https://finna.example.com
finna context use prod

# 2. Log in
finna login                  # username/password
finna login --oidc myidp     # OIDC / SSO
finna login --github         # GitHub OAuth

# 3. Check the dashboard
finna dashboard

# 4. View costs
finna costs summary --since 30d
finna costs breakdown --provider gcp --top 10
finna costs daily --days 14

# 5. Manage extractors
finna extractors list
finna extractors trigger my-extractor-id --wait
finna runs logs <run-id> --follow
```

---

## Commands

| Group | Commands |
|-------|----------|
| **auth** | `login`, `logout`, `whoami`, `auth register gcp\|azure`, `auth providers` |
| **context** | `context list\|use\|add\|remove\|current` |
| **configs** | `configs list\|get\|create\|update\|delete\|test` |
| **projects** | `projects list\|get\|create\|delete\|use` |
| **extractors** | `extractors list\|get\|register\|delete\|trigger` |
| **runs** | `runs list\|get\|cancel\|logs\|watch` |
| **costs** | `costs summary\|totals\|breakdown\|daily\|by-sku\|skus\|export\|list` |
| **dashboard** | `dashboard` (alias: `status`) |
| **alerts** | `alerts list\|stats\|ack\|ack-all` |
| **wastage** | `wastage summary\|findings\|rules\|scan` |
| **diag** | `ping`, `db-stats`, `version`, `debug curl` |

Run `finna <command> --help` for flags and examples on any subcommand.

---

## Configuration

Config file: `$XDG_CONFIG_HOME/finna/config.toml` (default `~/.config/finna/config.toml`).  
Tokens are stored in the **OS keyring** (macOS Keychain / libsecret), never written to disk.

```toml
current_context = "prod"

[contexts.prod]
  server          = "https://finna.example.com"
  default_project = "my-project"

[contexts.local]
  server  = "http://localhost:8000"
  insecure = true

[ui]
  color  = "auto"   # auto | always | never
  output = "table"  # table | json | yaml | csv | wide
```

### Environment overrides

| Variable | Description |
|----------|-------------|
| `FINNA_SERVER` | Override server URL for the current command |
| `FINNA_CONTEXT` | Override active context |
| `FINNA_TOKEN` | Inject a JWT directly (skips keyring) |
| `FINNA_OUTPUT` | Override output format |
| `FINNA_DEBUG` | Set to `1` to dump HTTP traffic (tokens redacted) |
| `NO_COLOR` | Disable all ANSI color output |

### Global flags

```
--context string   Use a specific context
--server string    Override server URL
--output, -o       Output format: table|json|yaml|csv|wide
--no-color         Disable color
--debug            Print HTTP requests/responses to stderr
--quiet            Suppress all output except errors
--no-input         Fail instead of prompting (CI-safe)
```

---

## Output formats

All list and get commands support `-o` / `--output`:

```sh
finna costs summary -o json   | jq '.total'
finna runs list -o csv        > runs.csv
finna extractors list -o wide
```

---

## Shell completion

```sh
finna completion bash   >> ~/.bashrc
finna completion zsh    >> ~/.zshrc
finna completion fish   > ~/.config/fish/completions/finna.fish
```

---

## Development

**Requirements:** Go 1.25+, `golangci-lint` v2.

```sh
git clone https://github.com/acarmisc/finna-cli
cd finna-cli
make build          # ./finna
make test           # unit tests with race detector
make lint           # golangci-lint run
make generate       # regenerate API client from api/openapi.yaml
make snapshot       # goreleaser snapshot (all platforms, no publish)
```

Install lint:
```sh
brew install golangci-lint
```

### Regenerating the API client

The OpenAPI spec is vendored at `api/openapi.yaml`. To refresh from the backend:

```sh
cp /path/to/finna-app/docs/openapi.yaml api/openapi.yaml
make generate
```

### Running against a local finna-app

```sh
docker-compose -f /path/to/finna-app/docker-compose.yml up -d
finna context add local --server http://localhost:8000
finna context use local
finna login
```

---

## License

MIT — see [LICENSE](./LICENSE).
