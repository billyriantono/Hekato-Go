# Hekato-Go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Hekato-Go is a multi-provider OpenAI / Anthropic compatible AI gateway: one endpoint in front of pools of Kiro, CodeBuddy and Grok accounts, with load balancing, smart routing, per-key limits and an operations dashboard.

[English](README.md) | [中文](README_CN.md)

> **Project origin**
>
> Hekato-Go started as an enhanced fork of [Quorinex/Kiro-Go](https://github.com/Quorinex/Kiro-Go).
> Thanks to the Quorinex/Kiro-Go maintainers for the original project.
> This fork is maintained independently at [billyriantono/Hekato-Go](https://github.com/billyriantono/Hekato-Go) and adds multi-provider support for Kiro and CodeBuddy.
> The Enterprise SSO / Azure AD support in this fork is based on code and ideas from [Quorinex/Kiro-Go PR #131](https://github.com/Quorinex/Kiro-Go/pull/131). Thanks to the PR author and discussion participants for making that work available.

If this project helps you, a Star would mean a lot.

## Features

- Anthropic `/v1/messages` & OpenAI `/v1/chat/completions`
- Multi-provider upstream routing: Kiro, CodeBuddy Global / China, Grok (xAI)
- Provider-neutral request layer: every client format is parsed once into a neutral form and each provider serializes from it, so adding an upstream is a new package plus one registration file
- Virtual `auto` model: requests are classified by complexity and routed to the best account/model by a learning bandit (see Auto routing)
- Multi-account pool with round-robin load balancing
- Auto token refresh, SSE streaming, Web admin panel
- Multiple Kiro auth methods: AWS Builder ID, IAM Identity Center (Enterprise SSO), SSO Token, local cache, credentials JSON
- CodeBuddy API-key account onboarding
- Usage tracking, account import/export, i18n (CN / EN)
- Support configuring outbound proxy (SOCKS5 / HTTP), plus a proxy pool that pins each account to a stable egress IP
- Per-API-key limits: token / credit quotas, requests per minute, max concurrent requests
- Conversation affinity: multi-turn sessions stick to the same upstream account so prompt caches stay warm
- Optional AES-256-GCM encryption of stored credentials (`ENCRYPTION_KEY`)
- Modern React admin dashboard (Vite + Tailwind + shadcn/ui + TanStack) with ops metrics (throughput, latency p50/p95, tokens, per-model / per-account breakdowns) persisted across restarts
- Public self-service usage page at `/usage`

## Quick Start

### Docker Compose (Recommended)

```bash
git clone https://github.com/billyriantono/Hekato-Go.git
cd Hekato-Go
mkdir -p data
docker-compose up -d
```

### Docker Run

```bash
docker run -d \
  --name hekato-go \
  -p 8080:8080 \
  -e ADMIN_PASSWORD=your_secure_password \
  -v /path/to/data:/app/data \
  --restart unless-stopped \
  ghcr.io/billyriantono/hekato-go:latest
```

### Build from Source

```bash
git clone https://github.com/billyriantono/Hekato-Go.git
cd Hekato-Go
go build -o hekato-go .
./hekato-go
```

The Go module is `hekato-go`; the admin dashboard bundle is committed under `web/` so no Node toolchain is needed to build or run. See "Admin Dashboard (development)" to rebuild it.

### Deploy on Zeabur

The repo already includes a `Dockerfile`, so it builds and runs on Zeabur out of the box.

**Option 1: Dashboard (one-click)**

1. Fork this repo to your GitHub account.
2. In Zeabur, create a new service and choose **Deploy from GitHub**, then select your fork.
3. Zeabur auto-detects the `Dockerfile` and builds the image.
4. In the **Networking** tab, expose port `8080` and bind a domain.
5. In the **Variables** tab, set at least `ADMIN_PASSWORD` (admin panel password).
6. Mount a Volume at `/app/data` if you want accounts / config to survive redeploys.

**Option 2: CLI**

```bash
npm i -g zeabur
zeabur auth login
zeabur deploy
```

> Run the commands from the project root. The CLI writes `.zeabur/context.json` to remember the target project / service — it contains personal IDs, so don't commit it.

Once the service is up, open `https://<your-domain>/admin` to log in.

Config is auto-created at `data/config.json`. Mount `/app/data` for persistence. The default admin password is `changeme` — override it via the `ADMIN_PASSWORD` env var or change it in the admin panel before going to production.

## Usage

Open `http://localhost:8080/admin`, log in, add accounts, then call the API:

```bash
# Claude
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4.5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}]}'

# OpenAI
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hello!"}]}'
```

## Thinking Mode

Append a suffix (default `-thinking`) to the model name, e.g. `claude-sonnet-4.5-thinking`. Claude-compatible requests that include a top-level `thinking` config such as `{"type":"enabled","budget_tokens":2048}` or `{"type":"adaptive"}` also enable thinking mode automatically. Configure output format in the admin panel under Settings - Thinking Mode.

## Outbound Proxy

For users in restricted network regions, configure an outbound proxy in the admin panel under **Settings - Outbound Proxy Settings**. Supports SOCKS5 and HTTP proxies.

The setting takes effect immediately without restarting.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFIG_PATH` | JSON config path and migration seed path. Also used to derive the default SQLite DB location. | `data/config.json` |
| `DB_DRIVER` | Storage backend: `json` / `file`, `sqlite` / `sqlite3`, or `postgres` / `postgresql` / `pgx`. Leave unset to keep the legacy JSON file backend. | `json` |
| `DATABASE_URL` | Database DSN. For SQLite, this can be a file path such as `/app/data/kiro.db`; if omitted with `DB_DRIVER=sqlite`, it defaults to `kiro.db` next to `CONFIG_PATH`. For PostgreSQL, use a URL such as `postgres://user:pass@host:5432/db?sslmode=disable` and it is required. | - |
| `ADMIN_PASSWORD` | Admin panel password. If unset on a fresh install, the first-run setup screen lets you create it in the browser. If set, it overrides the config value. | - |
| `KIRO_SSO_CALLBACK_BIND` | Bind address for the temporary Enterprise SSO callback listener. Useful in Docker when publishing the callback port. | loopback only |
| `KIRO_PROFILE_REGIONS` | Optional comma-separated region override for Kiro profile/usage probing. | built-in regions |
| `LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, or `error`. | `info` |
| `ENCRYPTION_KEY` | When set, account tokens, client secrets, relay secrets, API key values and the admin password are stored encrypted (AES-256-GCM). Existing plaintext data is sealed on the next save. Removing the key later makes the config unreadable, so keep it safe. | - |
| `ACCOUNT_REFRESH_MINUTES` | Fallback for how often every account's token and quota/credits are re-checked upstream. The value set in the dashboard (Settings → General) takes precedence. | `30` |

### Storage Backends

By default, Hekato-Go keeps backward-compatible single-file storage in `CONFIG_PATH`.
To use SQL storage instead:

```bash
# SQLite
DB_DRIVER=sqlite
# optional; defaults to kiro.db beside CONFIG_PATH
DATABASE_URL=/app/data/kiro.db

# PostgreSQL
DB_DRIVER=postgres
DATABASE_URL=postgres://user:pass@postgres:5432/hekato?sslmode=disable
```

When SQL storage is enabled for the first time, an existing `config.json` is used as the migration seed so current accounts/settings can be carried over.

## Auto routing

Enable it under Settings → Auto Routing, then send requests with `"model": "auto"`. Each request is classified by local signals (input tokens, tool count, turns, images, thinking) into a fast / balanced / strong tier. Inside the tier, every (account, model) pair the pool advertises is a candidate; a Thompson-sampling bandit over decayed success/failure, weighted by observed latency and remaining account quota, picks one. Quality and Cost sliders shift the tier, Speed weights latency, Explore sets the random-exploration rate. Tier membership is configured as substring patterns (`haiku`, `sonnet`, `opus`) or exact model IDs. Conversations already pinned to an account keep their model so upstream prompt caches stay warm. The resolved model is returned in `X-Hekato-Routed-Model`, and the last 200 decisions with the learned per-candidate stats are shown in the dashboard. When disabled, `auto` is passed through to the upstream unchanged.

## Ops metrics

The gateway keeps per-minute metrics (requests, errors, tokens, credits, latency histogram, per-model / per-account / per-endpoint counters) for 7 days. They are persisted through the active storage backend (a `metrics_minutes` table on SQLite/Postgres, or `metrics.json` beside `config.json`), restored at startup, flushed every 30 seconds and on graceful shutdown (SIGINT/SIGTERM). The Overview page shows them with 1h / 6h / 24h / 7d ranges.

## Documentation site

Every gateway serves its own documentation at `/docs`: endpoint reference, model names and thinking mode, the `auto` model, limits, and step-by-step setup guides for Claude Code, Codex CLI, Cursor, Cline / Roo Code, Continue, Aider, the OpenAI and Anthropic SDKs, LangChain, Open WebUI / LibreChat and curl. Snippets are pre-filled with the gateway's own base URL.

## Self-service usage check

Users can open `/usage` on the gateway, paste their own API key, and see that key's quota, limits and usage. The page calls `GET /v1/usage` with the key as `Authorization: Bearer`; no admin credentials are involved.

## Rate Limiting & Routing

Each API key can carry a token quota, a credit quota, a requests-per-minute limit and a max-concurrent-requests limit (0 = unlimited). Requests over a limit get `429` with `Retry-After`. Limits are configured per key in the admin panel.

Multi-turn conversations are pinned to the upstream account that served the first turn (keyed by model + system prompt + first user message, 5 minute sliding TTL), so upstream prompt caches are reused instead of being spread round-robin across accounts. If that account is cooling down or excluded, routing falls back to weighted round-robin and re-pins.

## Adding an upstream provider

1. Create `providers/<name>/` with the transport (`Call`), a `FromNeutral` serializer from `providers.NeutralChat`, model listing, usage fetch, and any admin auth routes registered via `providers.RegisterAdminRoutes` in `init()`.
2. Add the provider kind and account classification in `config/provider.go`.
3. Add `proxy/provider_<name>.go` calling `registerAdapter` with `chatFromClaude`, `chatFromOpenAI`, optional `responses`, `listModels`, `fetchUsage`.
4. Add the add-account flow in `dashboard/src/pages/accounts/add-account-dialog.tsx`.

The handlers, pool, rate limiting, affinity, auto routing and metrics never reference a provider directly.

## Admin Dashboard (development)

The dashboard source lives in `dashboard/` (Vite + React + TypeScript). The built output is committed to `web/` so the Go binary needs no Node toolchain at runtime.

```bash
cd dashboard
pnpm install
pnpm dev      # dev server on :5173, proxies /admin/api to :8080
pnpm build    # writes the production bundle into ../web
```

## Contributing

Friendly discussion is welcome. If you run into issues, try asking Claude Code, Codex, or similar tools for help first — most problems can be solved that way. PRs are even better.

## Friend Links

- [LINUX DO](https://linux.do)

## Disclaimer

For educational and research purposes only. Not affiliated with Amazon, AWS, Kiro, Tencent, or CodeBuddy. Hekato-Go is an independently maintained enhanced fork of Quorinex/Kiro-Go. Users are responsible for complying with applicable terms of service and laws. Use at your own risk.

## License

[MIT](LICENSE)
