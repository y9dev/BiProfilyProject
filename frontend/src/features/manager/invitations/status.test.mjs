import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  fromLocalInput,
  invitationActions,
  isPast,
  registrationUrl,
  remaining,
  statusTone,
  toLocalInput,
  validateMax,
} from './status.js'

const inv = (over) => ({ status: 'active', maxRegistrations: 10, registrationsCount: 3, ...over })

test('statusTone maps every status', () => {
  assert.equal(statusTone('active'), 'success')
  assert.equal(statusTone('expired'), 'warning')
  assert.equal(statusTone('full'), 'info')
  assert.equal(statusTone('revoked'), 'danger')
  assert.equal(statusTone('weird'), 'neutral')
})

test('actions: revoked invitations cannot be changed or revoked again', () => {
  const a = invitationActions(inv({ status: 'revoked' }))
  assert.deepEqual([a.reactivate, a.changeMax, a.revoke], [false, false, false])
})

test('actions: delete only when unused', () => {
  assert.equal(invitationActions(inv({ registrationsCount: 0 })).remove, true)
  assert.equal(invitationActions(inv({ registrationsCount: 1 })).remove, false)
})

test('actions: expired and full can be reactivated', () => {
  assert.equal(invitationActions(inv({ status: 'expired' })).reactivate, true)
  assert.equal(invitationActions(inv({ status: 'full' })).reactivate, true)
})

test('remaining never negative', () => {
  assert.equal(remaining(inv()), 7)
  assert.equal(remaining(inv({ registrationsCount: 12 })), 0)
})

test('validateMax: range and not below count', () => {
  assert.equal(validateMax('5', 3), null)
  assert.equal(validateMax('3', 3), null)
  assert.equal(validateMax('2', 3), 'below_count')
  assert.equal(validateMax('0', 0), 'out_of_range')
  assert.equal(validateMax('10001', 0), 'out_of_range')
  assert.equal(validateMax('abc', 0), 'out_of_range')
  assert.equal(validateMax('', 0), 'out_of_range')
  assert.equal(validateMax('1.5', 0), 'out_of_range')
})

test('registrationUrl encodes the code', () => {
  assert.equal(registrationUrl('https://x.kz', 'K7Q2M9XR4TBA'), 'https://x.kz/register?code=K7Q2M9XR4TBA')
  assert.equal(registrationUrl('http://localhost:5193', 'a b'), 'http://localhost:5193/register?code=a%20b')
})

test('local datetime round trip', () => {
  assert.equal(toLocalInput(null), '')
  assert.equal(fromLocalInput(''), null)
  assert.equal(fromLocalInput('garbage'), undefined)
  const iso = '2030-05-04T10:30:00.000Z'
  assert.equal(fromLocalInput(toLocalInput(iso)), iso)
})

test('isPast', () => {
  assert.equal(isPast(null), false)
  assert.equal(isPast('2000-01-01T00:00:00Z'), true)
  assert.equal(isPast('2999-01-01T00:00:00Z'), false)
})
