import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dashboardVariant, isStaffVariant } from './dashboardVariant.js'

test('employees never get a staff variant', () => {
  assert.equal(dashboardVariant({ role: 'employee', completed: 0 }), 'employee-gate')
  assert.equal(dashboardVariant({ role: 'employee', completed: 3 }), 'employee')
  for (const role of ['employee', undefined, null, '', 'superuser', 'Admin']) {
    for (const completed of [0, 1, 7, undefined]) {
      assert.equal(isStaffVariant(dashboardVariant({ role, completed })), false, `${role}/${completed}`)
    }
  }
  assert.equal(dashboardVariant(), 'employee-gate')
})

test('manager and admin get staff / staff-gate', () => {
  for (const role of ['manager', 'admin']) {
    assert.equal(dashboardVariant({ role, completed: 0 }), 'staff-gate')
    assert.equal(dashboardVariant({ role, completed: 2 }), 'staff')
    assert.ok(isStaffVariant(dashboardVariant({ role, completed: 0 })))
  }
})
