# mflint

Lints Kubernetes manifests for security/reliability issues and estimates their monthly cloud
cost — a CLI, an HTTP API, and a paste-YAML web page, all sharing one Go core. Cost figures are
static list-price estimates, not a guaranteed bill.

## Layout

- `cmd/mflint` — CLI (`mflint <path> --format=table|json`)
- `cmd/server` — API + web server
- `internal/parser` — YAML → typed Kubernetes objects
- `internal/rules` — 13 lint rules (4 critical, 6 warning, 3 info); see each `internal/rules/k8s_*.go` file
- `internal/cost` — monthly cost estimator (AWS/GCP/Azure static pricing tables)
- `internal/report` — combines violations + cost into table/JSON output
- `internal/store` — Postgres (users, saved reports, subscriptions, checkout sessions, monthly
  usage, API keys)
- `internal/auth` — OAuth-only sign-in (Google + GitHub, see below) + JWT sessions; no passwords.
  Also mints/hashes long-lived API keys (`mflint_...`, see below) — a separate concept from a
  session JWT, told apart by prefix.
- `internal/billing` — `Provider` interface; `CoinGateProvider` (crypto, see below) is registered
  when configured, falling back to `NOWPaymentsProvider` then `NoopProvider`. `StripeProvider` is a
  shape/TODO only — Stripe doesn't support Iran-based accounts.
- `internal/api` — chi HTTP API
- `web/` — `landing.html` (marketing, at `/`), `index.html` (the tool + API key panel, at `/app`),
  `login.html` (OAuth sign-in, at `/login`), `docs.html` (API reference, at `/docs`) — all served
  by `cmd/server`
- `action.yml` — GitHub Action wrapping the CLI for PR comments (lives at repo root — GitHub
  Marketplace requires that)
- `testdata/manifests/` — sample good/bad manifests

## CLI

The CLI is a thin client of the hosted API — every run sends the manifest to `/v1/lint` and
prints back the same report the web app would show. It does **not** lint locally / offline.
Anonymous runs are only IP rate-limited (same as the web's free-form box); a logged-in run
additionally counts against that account's plan and monthly quota
(`billing.MonthlyCheckLimit` — free 50/mo, team 1000/mo, pro unlimited), identically to the web.

```sh
go run ./cmd/mflint testdata/manifests/bad.yaml --format=table
go run ./cmd/mflint testdata/manifests/good.yaml --format=json

# save an API key locally (generate one at <api-base>/app -> API keys) so runs count
# against your plan instead of running anonymous/rate-limited:
go run ./cmd/mflint login
go run ./cmd/mflint whoami          # plan + this month's usage
go run ./cmd/mflint logout
```

Flags: `--format table|json` (default table), `--cloud aws|gcp|azure` (default aws), `--no-cost`
to skip cost estimation, `--api-base` (or `MFLINT_API_BASE`) to point at a non-default server,
`--token` (or `MFLINT_API_KEY`) to use a key for one run without `login` (handy for CI — the
GitHub Action below does exactly this). Exits 1 if any CRITICAL violation is found (for CI
gating), 2 on any error (network, invalid key, quota exceeded), 0 otherwise.

`login` saves the key to `$XDG_CONFIG_HOME/mflint/config.json` (`~/.config/mflint/config.json`
on Linux/most setups), mode 0600, alongside the resolved `--api-base`. The default API base is
the `defaultAPIBase` constant in `cmd/mflint/api.go`, currently `https://cubzi.cloud` (the live
deployment — see "Deploying to a VPS" below).

## Running the server locally

```sh
docker compose up -d postgres          # local Postgres
export DATABASE_URL="postgres://mflint:mflint@localhost:5432/mflint?sslmode=disable"
export JWT_SECRET="dev-secret-change-me"
go run ./cmd/server                    # listens on :8080, serves web/ at "/"
```

Without `DATABASE_URL` set, the server still runs and `/v1/lint` still works — auth, saved
reports, and billing routes just aren't registered.

### API

- `POST /v1/lint` — public, no auth required. Body: raw YAML, or JSON
  `{"manifest": "...", "cloud": "aws", "save": true}`. Anonymous calls are only IP rate-limited.
  A valid `Authorization: Bearer <token>` additionally counts against that account's monthly quota
  (`billing.MonthlyCheckLimit` — free 50/mo, team 1000/mo, pro unlimited) and returns `429` once
  it's exceeded; `save: true` + auth also persists the report.
- `GET /v1/auth/providers` — public, `{"google": bool, "github": bool}` — which sign-in options
  are actually configured; `web/login.html` uses this to disable an unconfigured button.
- `GET /v1/auth/google/login`, `GET /v1/auth/github/login` — redirect to the provider's consent
  screen. Only registered once that provider's client id + secret are set (see below).
- `GET /v1/auth/google/callback`, `GET /v1/auth/github/callback` — exchange the auth code,
  create/find the user by email, and redirect to `/app?token=<jwt>`.
- `GET /v1/reports` — auth required, lists the caller's saved reports
- `GET /v1/usage` — auth required, `{"plan", "limit", "used", "remaining"}` for the current month
- `GET /v1/billing/plan` — auth required, current plan (defaults to `free`)
- `POST /v1/billing/checkout` — auth required, `{"plan": "team"|"pro"}`; creates a CoinGate order
  (or NOWPayments invoice) and returns `{"url", "orderId"}` when configured (see below), 501
  otherwise
- `POST /v1/billing/webhook/coingate` — public; on a callback, re-fetches the order from
  CoinGate's authenticated API rather than trusting the callback body itself (see
  `CoinGateProvider`'s doc comment), then upgrades the paying user's plan if it's paid.
- `POST /v1/billing/webhook/nowpayments` — public; authenticates via the `x-nowpayments-sig` HMAC
  header instead of a bearer token. Upgrades the paying user's plan on a confirmed payment.
- `GET /v1/api-keys`, `POST /v1/api-keys` (`{"label"}`), `DELETE /v1/api-keys/{id}` — auth
  required. Full reference and `curl` examples at `/docs`.

Full endpoint-by-endpoint reference with request/response examples: `/docs` (served by
`cmd/server`, source at `web/docs.html`).

### API keys

A session JWT (from signing in) and an API key (`mflint_...`, minted from the panel at `/app` →
"API keys") both work as `Authorization: Bearer <token>` — they resolve to the same account and
the same plan/quota (`billing.MonthlyCheckLimit`). Use a session token for the browser and an API
key for scripts/CI, since it doesn't expire. The raw key is shown exactly once at creation time;
only its SHA-256 hash is stored (`internal/auth.HashAPIKey`).

### Sign-in (Google + GitHub OAuth only, no passwords)

```sh
export GOOGLE_OAUTH_CLIENT_ID="..."
export GOOGLE_OAUTH_CLIENT_SECRET="..."
export GOOGLE_OAUTH_REDIRECT_URL="https://your-domain/v1/auth/google/callback"

export GITHUB_OAUTH_CLIENT_ID="..."
export GITHUB_OAUTH_CLIENT_SECRET="..."
export GITHUB_OAUTH_REDIRECT_URL="https://your-domain/v1/auth/github/callback"
```

Create the OAuth app yourself (I can't do this step for you):
- Google: [Google Cloud Console](https://console.cloud.google.com/apis/credentials) → Create
  Credentials → OAuth client ID → Web application. Add the redirect URL above under "Authorized
  redirect URIs".
- GitHub: [github.com/settings/developers](https://github.com/settings/developers) → New OAuth
  App. Set "Authorization callback URL" to the redirect URL above.

Either provider works independently — you don't need both. With neither configured, `/login`
still loads but shows both buttons disabled (via `GET /v1/auth/providers`).

### Crypto billing (CoinGate)

Set these env vars to make `/v1/billing/checkout` actually work:

```sh
export COINGATE_API_KEY="..."
export COINGATE_API_BASE="https://api-sandbox.coingate.com"   # sandbox; https://api.coingate.com for production
export COINGATE_CALLBACK_URL="https://your-domain/v1/billing/webhook/coingate"
export COINGATE_SUCCESS_URL="https://your-domain/app"          # optional
export COINGATE_RECEIVE_CURRENCY="USD"                          # asset settled into your CoinGate balance
```

Get an API key from your account's API settings at **sandbox.coingate.com** (free, no real funds
needed) or **coingate.com** for production. I can't create this account for you; it's a manual
signup step. Team is priced at $19, Pro at $49 (`internal/billing/nowpayments.go`'s
`planPriceUSD` — shared by both providers; edit alongside the pricing table in the original spec
if it changes).

Unlike most webhook-based integrations, CoinGate's callback body isn't itself signed — see
`CoinGateProvider`'s doc comment (`internal/billing/coingate.go`) for how `HandleWebhook`
establishes trust by re-fetching the order from CoinGate's authenticated API instead.

NOWPayments is still supported as a fallback (`NOWPAYMENTS_API_KEY` etc., used only when
`COINGATE_API_KEY` is unset) — the two were evaluated side by side and CoinGate is the one
actually configured on the live deployment, but both implementations are equally real/tested.

## Deploying to a VPS

`Dockerfile` builds `cmd/server` (a static binary + `web/`). `deploy/docker-compose.yml` runs it
alongside Postgres and a Caddy reverse proxy that gets a free TLS cert automatically (just needs
a domain's DNS A record pointed at the VPS).

```sh
# on the VPS, with Docker + the Docker Compose plugin installed and a domain's
# A record already pointing at this machine's IP:
git clone <your-repo-url> mflint && cd mflint/deploy
cp .env.example .env
# edit .env: DOMAIN, POSTGRES_PASSWORD, JWT_SECRET (openssl rand -hex 32), and
# whichever of the OAuth/NOWPayments vars you have real credentials for yet —
# every one is optional at the server level; only the features that need it
# stay disabled without it (see "What's stubbed on purpose" below)
docker compose up -d --build
```

**This is live**: deployed at `https://cubzi.cloud` (a 1 vCPU / 1 GB VPS — fine for early traffic,
revisit sizing once real usage shows up), TLS issued automatically by Caddy. `cmd/mflint/api.go`'s
`defaultAPIBase` and `action.yml`'s `api-base` default both already point at it. Google/
GitHub OAuth and NOWPayments are still unset there (see "What's stubbed on purpose" below) —
sign-in and paid checkout stay disabled until those env vars are added to the VPS's `deploy/.env`
and the stack is restarted (`docker compose up -d`).

Note: `docker build` could not be tested from this dev environment before the VPS existed — Docker
Hub (`registry-1.docker.io`) returns `403 Forbidden` on every pull from this machine's network
(confirmed with plain `docker pull alpine`, unrelated to this project's Dockerfile), most likely a
sanctions-related geo-block on this connection. It built and runs fine on the VPS itself, which has
normal (non-Iran) egress.

## GitHub Action

```yaml
- uses: Mehrshadshaabani/kubzimanifest@v1
  with:
    path: ./manifests
    cloud: aws
```

Builds and runs the CLI, then posts/updates a single PR comment with the table report; fails the
job if any CRITICAL violation is found. Since the CLI now calls the hosted API for every run, pass
`api-key: ${{ secrets.MFLINT_API_KEY }}` (an API key from `<api-base>/app` -> API keys) so CI runs
count against a real plan instead of racing other traffic on the anonymous IP rate limit — without
it, a busy shared runner IP can start getting 429s.

## Testing

```sh
go test ./...
```

Every rule has a table-driven test (bad manifest → ≥1 violation, good manifest → 0). The cost
estimator and parser have their own unit tests.

## What's stubbed on purpose

- **Card payments**: `internal/billing.StripeProvider` exists only as a shape/TODO — Stripe
  doesn't operate in Iran. CoinGate (crypto, with NOWPayments as a tested fallback) is what's
  actually wired in when configured.
- **Google/GitHub OAuth apps**: the code is complete and wired in, but each provider only turns on
  once you've created that OAuth app yourself and set its env vars — see "Sign-in" above.
- **Email delivery**: not addressed — there's no email-based flow to begin with (OAuth-only
  sign-in), so nothing currently needs it.
