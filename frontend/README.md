# BI Profily frontend

React 19 + Vite + Tailwind 4 SPA (ru/kk). See the repository root README for the full picture.

```
npm install
npm run dev     # Vite dev server, proxies /api to the backend
npm run check   # i18n + legacy guards + unit tests
npm run lint    # oxlint
npm run build
```

## Analytics (`/analytics`, admin only)

Lazy route inside the admin `RequireRole` block (`src/App.jsx`); the sidebar link is shown to admins
only, and managers/employees get the shared forbidden page. The backend enforces the same rule
(`/api/analytics/*` is admin-only). Code lives in `src/features/analytics/`.

- **URL state** (`analyticsState.js`, pure, tested by `analyticsState.test.mjs`):
  shared filter `?period=7d|30d|90d|12m|all|custom&from=YYYY-MM-DD&to=YYYY-MM-DD&c=1&c=7`
  (default `90d`; `custom` needs both dates; `c` repeated, max 10; no `c` = server default
  selection = 4 core + top-3 by coverage, `defaultIds`). Every tab sends this filter unchanged.
  Tab-local keys: `tab`, `dep` (repeated, 1-8 compared departments or `none`), the department
  population filter per tab (`pdept` Overview participation, `edept` Employees, `ddept` Data tab
  ranking + changes; each is sent as `departmentId` and never leaks to another tab), `emp`, `q`,
  `data=all|no_data|stale`, `thr` (1-100, default 15). Invalid values fall back to defaults;
  unknown characteristic ids rejected by the server (422) reset `c`; compared departments that no
  longer exist are dropped from `dep` (URL replaced, no dead end).
- **Tabs:** Overview (`CompanyTrendCard` in `analytics` mode + `ParticipationPanel`), Departments
  (`DepartmentsView`: one `StockChart` per characteristic with the company reference line),
  Employees (`EmployeeView`: picker / no-data list and the employee view), Data & rankings
  (`RankingPanel`, `ChangesPanel`, `DataCoveragePanel`).
- **Colours:** core characteristics `--core-1..4` (solid, by id), additional ones `--extra-1..6`
  (dashed, in selection order).
- Groups below 5 employees show "not enough data" with n; every aggregate shows its sample size.
- Tables reuse `features/manager/DataTable.jsx` (cross-feature import, intentionally not moved).
- Strings: `src/i18n/ns/analytics.{ru,kk}.json` (page) and `stats.{ru,kk}.json` (shared with the dashboard).
