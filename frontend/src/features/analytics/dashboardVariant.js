// Which dashboard a user gets. Pure (unit-tested in dashboardVariant.test.mjs); Dashboard.jsx branches only on this.
//   'employee'      personal dashboard (unchanged)
//   'employee-gate' first-test gate (unchanged)
//   'staff'         manager/admin: company trend instead of StatsPanel, team status instead of TrendsPanel
//   'staff-gate'    manager/admin before their own first test: the gate layout, with the company cards
//                   instead of StatsLocked (plan open question (a), default)
const STAFF_ROLES = ['manager', 'admin']

export function dashboardVariant({ role, completed } = {}) {
  const gate = !(Number(completed) > 0)
  if (STAFF_ROLES.includes(role)) return gate ? 'staff-gate' : 'staff'
  return gate ? 'employee-gate' : 'employee'
}

export const isStaffVariant = (v) => v === 'staff' || v === 'staff-gate'
