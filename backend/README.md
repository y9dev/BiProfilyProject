# Backend: BI Profily

Go service (net/http, pgx, Squirrel, Redis) for the BI Profily employee assessment platform.
Requirements: [`docs/REQUIREMENTS.md`](../docs/REQUIREMENTS.md). Plan: `.omc/plans/bi-profily-redesign.md`.

## Layout

```
cmd/server/main.go     entry point: config + production checks -> reset gate -> PG -> Redis -> migrations -> admin bootstrap -> HTTP
internal/config        environment variables
internal/store         pgxpool + go-redis connections
internal/migrate       embedded SQL migrations, tracked in schema_migrations
internal/model         domain types (mirror the schema)
internal/auth          argon2id passwords, Redis sessions, middleware (auth, avatar gate, role, CSRF, rate limit), audit
internal/users         user repository
internal/invitations   invitations.Repository (frozen interface; implemented in Steps 3B and 5B)
internal/admin         admin read side (user search, audit)
internal/httpapi       routing (routes.go: protected()/public() + route registry), handlers
internal/httpapi/dto   frozen HTTP contract: request/response structs with json tags
```

## Database and migrations

- `000_guard.sql` aborts on a legacy (pre-redesign) schema: "legacy schema detected: reset volume (docker compose down -v) or run with MIGRATE_RESET=1 in dev".
- `001_schema.sql` is the baseline: departments, invitations, users (login, avatar), characteristics, assessments, questions, options, option_effects (immutable via trigger), attempts (one per user and assessment), attempt_answers, attempt_scores, audit_log.
- `002_seed.sql` inserts the 4 protected default characteristics with ru/kk names.
- Each file runs once in a transaction and is recorded in `schema_migrations`; a second boot logs `migrate: applied=[] skipped=[...]`.
- The first admin is created in Go from `ADMIN_LOGIN` / `ADMIN_PASSWORD`, only when `users` is empty.

### Reset (dev only)

The redesign does not migrate old data. To move an existing volume to the new baseline:

```powershell
docker compose down -v          # drops the pgdata volume (take ops/backup.sh first if needed)
docker compose up -d
```

Or, without dropping the volume: `APP_ENV=dev MIGRATE_RESET=1` runs `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` before migrating.
With `APP_ENV` missing or any value other than `dev`, `MIGRATE_RESET=1` is refused with a fatal log before any DB connection.

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `HTTP_ADDR` | `:8080` | listen address |
| `PG_DSN` | `postgres://postgres:postgres@localhost:5432/bigroup?sslmode=disable` | Postgres |
| `REDIS_ADDR` | `localhost:6379` | Redis |
| `REDIS_PASSWORD` | empty | Redis `requirepass` (AUTH); empty = no password. Both compose files pass the same value to Redis and the backend |
| `MIGRATE` | `1` | apply migrations on start |
| `APP_ENV` | empty | environment marker, **required by docker compose**. `dev` allows `MIGRATE_RESET` and skips the production checks below; anything else (also empty) is production |
| `MIGRATE_RESET` | `0` | `1` drops and recreates the public schema on start (requires `APP_ENV=dev`) |
| `FRONT_ORIGIN` | `http://localhost:5173` | CORS origin (exact, cookies need it). Outside dev it must be `https://` or the server refuses to start |
| `COOKIE_SECURE` | `1`, or `0` when `APP_ENV=dev` | `1` for Secure cookies (HTTPS) |
| `TRUSTED_PROXIES` | empty | comma-separated CIDRs/IPs of reverse proxies. `X-Forwarded-For` is honoured only when the TCP peer is in this list (right-most untrusted hop); otherwise the client is `RemoteAddr` |
| `ADMIN_LOGIN` | `admin` | bootstrap admin login |
| `ADMIN_PASSWORD` | empty | bootstrap admin password, **required by the root docker compose**; no admin is created without it. Outside dev it must have 12+ characters and not be `admin`/`admin123`, or the server refuses to start |

Startup order: config load and production checks (`Validate`) -> reset gate -> PG -> Redis -> migrations -> admin bootstrap -> HTTP.
A refused configuration fails before any connection. The HTTP server uses `ReadHeaderTimeout 5s`, `ReadTimeout 15s`, `WriteTimeout 30s`, `IdleTimeout 60s`.

`backend/docker-compose.yml` (infrastructure for `go run` from the host) binds Postgres and Redis to `127.0.0.1` only and enables
Redis `requirepass` when `REDIS_PASSWORD` is set (`$env:REDIS_PASSWORD="..."; docker compose up -d`, then run the backend with the same value).

## Security

- Every response carries `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`,
  a deny-all `Content-Security-Policy` (`default-src 'none'; frame-ancestors 'none'; ...`, the API serves JSON only) and a restrictive `Permissions-Policy`.
- Client IP (`auth.ClientIP`) is the single source for rate limits, lockout and audit; see `TRUSTED_PROXIES`.
  Rate-limiter entries idle longer than the full bucket refill time (at least 10 min) are evicted.
- Login lockout: 5 failures per login **and IP** lock that pair for 15 min; 50 failures per login from all IPs in the
  15 min window lock the login. A successful login clears only its own login+IP counter.
- `GET /api/health` returns only `{"status":"ok"|"degraded"}`; causes are logged, never returned.
- Demoting or blocking an admin locks the active admin rows (`SELECT ... FOR UPDATE`) in a transaction, so two admins
  demoting each other concurrently cannot leave the system without an active admin (one gets 409 `last_admin`).
- `GET /api/admin/users?limit=` is capped at 200.

## Tests

```powershell
go build ./...; go vet ./...
go test ./internal/httpapi/dto/...      # contract (no infrastructure)
$env:TEST_PG_DSN="postgres://postgres:postgres@localhost:5432/bigroup_test?sslmode=disable"
$env:TEST_REDIS_ADDR="localhost:6379"      # test Redis without requirepass
go test ./... -count=1 -p 1
```

Integration tests (`internal/httpapi`, `internal/migrate`) skip when Postgres or Redis is unreachable.
`internal/httpapi` tests migrate and truncate the target database: point `TEST_PG_DSN` at a test database.
`internal/migrate` tests need an explicit `TEST_PG_DSN` and run in throwaway databases they create and drop.

## Route registration

Every `/api/*` route goes through `routes.go`:

- `s.public(pattern, handler)` only for the allowlist `GET /api/health`, `POST /api/auth/login`, `POST /api/auth/register`, `GET /api/register/options`.
- `s.protected(pattern, handler, routeOpts{roles, avatarExempt})` composes `RequireAuth -> RequireProfileComplete (unless avatarExempt) -> RequireRole -> RequireCSRF (every non-GET)`.
- Avatar-exempt allowlist: `GET /api/auth/me`, `POST /api/auth/logout`, `PUT /api/me/avatar`, `GET /api/avatar/catalog`.
- Direct `s.mux.Handle` for `/api/*` is forbidden. `routes_test.go` checks the registry against the contract below and sweeps every gated route with a user that has no avatar.
- The avatar gate re-reads `avatar_set_at` when the session flag is false and upgrades the session in place (Redis `SET XX KEEPTTL`).

## API contract (frozen)

Frozen in Step 1.3. The DTOs are in `internal/httpapi/dto/dto.go`; one fixture per DTO is in
`contracts/golden/*.json`, checked byte-for-byte by `internal/httpapi/dto/golden_test.go` and by shape in
`frontend/src/api/contract.test.mjs`. **Any contract change updates the golden file, the DTO and the frontend parser in one commit**
(`go test ./internal/httpapi/dto/... -update` regenerates the fixtures).

Conventions: JSON; errors `{"error": code, "message"?, "errors"?}` (validation: `{"error":"validation","errors":[{path, code}]}`);
CSRF header `X-CSRF-Token` on all non-GET authenticated routes. **Gate** = avatar gate applies.
Role `auth` = any signed-in active user; `mgr` = manager or admin; `admin` = admin only.

| Method & Path | Role | Gate | Body / Query | Success | Notable errors |
|---|---|---|---|---|---|
| GET `/api/health` | public | - | - | 200 `{status}` only | - |
| GET `/api/register/options?code=` | public, rate-limited | - | - | 200 `{valid, reason?, departments:[{id,name}]}` (departments only when valid) | 429 |
| POST `/api/auth/register` | public, rate-limited | - | `{code, login, password(8+), fullName, jobTitle, departmentId?}` | 201 + session (`profileComplete:false`) | 422 field errors; 409 `login_taken`; 410 `invite_invalid` (one generic reason code for expired/full/revoked/unknown) |
| POST `/api/auth/login` | public, rate-limited | - | `{login, password}` | 200 user | 401 generic; 403 blocked; 429 |
| POST `/api/auth/logout` | auth | exempt | - | 204 | - |
| GET `/api/auth/me` | auth | exempt | - | 200 `{id, login, fullName, jobTitle, department, role, avatar, profileComplete}` | 401 |
| GET `/api/avatar/catalog` | auth | exempt | - | 200 universes/genders/styles | - |
| PUT `/api/me/avatar` | auth | exempt | `{universe, gender, style}` | 200 | 422 not in catalog |
| GET `/api/me/profile` | auth | yes | - | 200 `{characteristics:[{id,name,nameI18n,mean,count}], timeline:[{attemptId, assessmentTitle, completedAt, scores}], stats:{completed, available}}` | - |
| GET `/api/assessments` | auth | yes | - | 200 visible assessments + `{completed, attemptId?}` per user | - |
| GET `/api/assessments/{id}/take` | auth | yes | - | 200 questions + options, **no effects** | 404 not visible; 409 `already_completed` |
| POST `/api/assessments/{id}/submit` | auth | yes | `{answers:[{questionId, optionIds}]}` | 201 `{attemptId, scores}` | 409 `already_completed` (UNIQUE); 404 not visible; 422 invalid answers |
| GET `/api/me/attempts/{id}` | auth (owner only) | yes | - | 200 result | 404 if not owner (no IDOR) |
| GET `/api/characteristics` | mgr | yes | - | 200 with `isDefault`, `inUse` | 403 |
| POST `/api/characteristics` | mgr | yes | `{name}` | 201 | 409 duplicate name |
| PUT `/api/characteristics/{id}` | mgr | yes | `{name}` | 200 | 409 `default_protected` (defaults cannot be renamed or deleted); 409 duplicate name |
| DELETE `/api/characteristics/{id}` | mgr | yes | - | 204 | 409 `default_protected` / `in_use` |
| GET `/api/departments` | mgr | yes | `?includeInactive=1` | 200 with `userCount`, `active` | 403 |
| POST `/api/departments` | mgr | yes | `{name}` | 201 | 409 duplicate |
| PUT `/api/departments/{id}` | mgr | yes | `{name}` | 200 | 409 duplicate |
| DELETE `/api/departments/{id}` | mgr | yes | - | 204 hard delete if unused; 200 `{deactivated:true}` if in use | - |
| POST `/api/departments/{id}/activate` | mgr | yes | - | 200 | 409 name clash |
| GET `/api/invitations` | mgr | yes | - | 200 `[{id, code, url, expiresAt, maxRegistrations, registrationsCount, status, createdAt, createdBy}]` | 403 |
| POST `/api/invitations` | mgr | yes | `{expiresAt: ISO\|null, maxRegistrations}` | 201 | 422 past date / max<1 |
| PATCH `/api/invitations/{id}` | mgr | yes | `{expiresAt?: ISO\|null, maxRegistrations?}` (reactivate = future expiresAt) | 200 | 409 `max_below_count`; 409 `revoked`; 422 past date |
| POST `/api/invitations/{id}/revoke` | mgr | yes | - | 200 | - |
| DELETE `/api/invitations/{id}` | mgr | yes | - | 204 | 409 `has_registrations` |
| GET `/api/invitations/{id}/registrations` | mgr | yes | - | 200 `[{fullName, jobTitle, department}]` | - |
| GET `/api/manage/assessments` | mgr | yes | - | 200 all non-deleted with status and attempt count | 403 |
| POST `/api/manage/assessments` | mgr | yes | full definition incl. effects | 201 `{id, warnings:[...]}` (`min_unreachable`, non-blocking) | 422 validation list |
| GET `/api/manage/assessments/{id}` | mgr | yes | - | 200 full definition incl. effects (read-only) | 404 |
| PATCH `/api/manage/assessments/{id}` | mgr | yes | `{title?, description?}` | 200 | 422 |
| PUT `/api/manage/assessments/{id}/visibility` | mgr | yes | `{hidden}` | 200 | - |
| PUT `/api/manage/assessments/{id}/expiry` | mgr | yes | `{expiresAt: ISO\|null}` | 200 | 422 |
| DELETE `/api/manage/assessments/{id}` | mgr | yes | - | 204. Hard delete if no attempts. If attempts exist: archive (`deleted_at`), hidden from employees and from the manager list, results kept in profiles/timelines | 404 |
| GET `/api/admin/users` | admin | yes | `?search=&role=&status=&limit=(<=200)&offset=` | 200 | 403 |
| PUT `/api/admin/users/{id}/role` | admin | yes | `{role: manager\|employee}` | 200 + sessions revoked | 409 `self_change` / `last_admin`; 422 |
| POST `/api/admin/users/{id}/block` and `/unblock` | admin | yes | - | 200 + sessions revoked on block | 409 `self_block` |
| POST `/api/admin/users/{id}/reset-password` | admin | yes | `{password(8+)}` | 204 + `DestroyByUser` (all sessions revoked) | 422; 404 |

Removed with the redesign: the legacy unauthenticated `/api/questions*`, `/api/evaluate`, `/api/results/*`,
`/api/users/{userId}/latest`, and `/api/traits*`, `/api/tests*`, `/api/attempts*`, `/api/profile`, `/api/users*`,
`/api/invites*`, `/api/admin/units*`, `/api/admin/users` POST/deactivate, and PDF export.

DTO and golden file per row: `me.json` (me, login, register, avatar PUT), `register_options*.json`, `avatar_catalog.json`,
`profile.json`, `assessments_list.json`, `take_view.json`, `submit_request.json`, `submit_result.json`, `attempt_result.json`,
`characteristics.json`, `departments.json`, `department_delete.json`, `invitation.json`, `invitations_list.json`,
`registrations.json`, `manage_assessments_list.json`, `manage_assessment.json`, `create_assessment_request.json`,
`create_assessment_result.json`, `admin_users.json`, `error.json`, `error_validation.json`.

## Operations (ops/)

- `ops/backup.sh`: nightly backup (`pg_dump -Fc` + `pg_dumpall --globals-only`), sha256 manifest, rotation.
- `ops/restore.sh`: trial restore with checksum verification.
- `ops/README.md`: operations guide.
