# BI Profily

Employee assessment platform: managers build assessments whose answers shift employee characteristics (e.g. the four default ones), employees take them once and see their profile, admins manage users. UI languages: ru and kk.

## Stack

- Backend: Go (net/http, pgx, Squirrel), PostgreSQL 16, Redis 7 (sessions, rate limit, cache).
- Frontend: React 19, Vite, Tailwind 4, react-router, i18n (ru/kk).
- Deployment: `docker-compose.yml` (postgres, redis, backend), `frontend/Dockerfile` + `nginx.conf`, `ops/` backup and restore scripts.

## Structure

- `backend/` - Go service, migrations, HTTP API. Details, environment variables and route rules: [`backend/README.md`](backend/README.md).
- `frontend/` - SPA (`src/features/*` per role, `src/ui` shared components, `src/i18n` dictionaries, `scripts/` guards).
- `contracts/golden/` - golden JSON files shared by backend DTO tests and frontend contract tests.
- `docs/` - [`REQUIREMENTS.md`](docs/REQUIREMENTS.md), [`INTEGRATION.md`](docs/INTEGRATION.md), [`VERIFICATION.md`](docs/VERIFICATION.md) (acceptance criteria and evidence).
- `ops/` - backup/restore scripts.

## Roles

- `employee` - registers (with or without an invitation code), picks an avatar (required before anything else), takes each assessment once, sees results and profile dynamics.
- `manager` - invitations (with QR), assessments (wizard), characteristics, departments. The dashboard shows the company trend of the core characteristics and Team status (participation, per-test completion, employees needing attention); values from fewer than 5 employees are hidden.
- `admin` - everything above plus user management (roles, blocking), audit and the Analytics page (`/analytics`: trends with any characteristics and periods, participation, department comparison, employee view, ranking, significant changes). On the dashboard an admin also sees small-sample values (dimmed).

The first admin is created at start from `ADMIN_LOGIN` / `ADMIN_PASSWORD` when the users table is empty.

## Run

Docker:

```
cp .env.example .env     # set APP_ENV and ADMIN_PASSWORD (both required); never commit .env
docker compose up -d
```

`docker compose` refuses to start without `APP_ENV` and `ADMIN_PASSWORD`. For local work use `APP_ENV=dev` (http origin, non-Secure cookies, any admin password).
Any other value (`prod`, `staging`) makes the backend refuse to start unless `FRONT_ORIGIN` is `https://` and `ADMIN_PASSWORD` has 12+ characters and is not `admin`/`admin123`; cookies are then Secure by default.

Environment variables (`APP_ENV`, `ADMIN_*`, `MIGRATE_RESET`, proxy and Redis settings) are described in [`backend/README.md`](backend/README.md) and `.env.example`. `MIGRATE_RESET=1` drops the schema and works only with `APP_ENV=dev`; it is a development-only reset.

Local development: start Postgres and Redis (e.g. the compose services), then

```
cd backend && go run ./cmd/server
cd frontend && npm install && npm run dev      # http://localhost:5173
```

## Tests

- Backend: `cd backend && go build ./... && go vet ./... && go test ./... -p 1`. Integration tests need `TEST_PG_DSN` and `TEST_REDIS_ADDR` (they skip when unreachable).
- Frontend: `cd frontend && npm run check` (i18n guard, legacy guard, unit tests), `npm run lint`, `npm run build`.
