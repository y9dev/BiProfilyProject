// One function per backend route (frozen contract: backend/README.md, contracts/golden/*.json).
// Return values are the response bodies exactly as documented; shapes are checked
// against the golden files by contract.test.mjs.
import { api, BASE_URL } from './client'

const id = (v) => encodeURIComponent(v)

export const auth = {
  me: (opts) => api.get('/api/auth/me', opts),
  login: (login, password) => api.post('/api/auth/login', { login, password }),
  logout: () => api.post('/api/auth/logout'),
  registerOptions: (code, opts) => api.get('/api/register/options', { ...opts, query: { code } }),
  register: (input) => api.post('/api/auth/register', input),
}

// Public, server-rendered profile widget (an HTML fragment, not JSON): only the URL is built here.
export const widget = {
  url: (userId, lang) => `${BASE_URL}/api/widget/${id(userId)}${lang ? `?lang=${encodeURIComponent(lang)}` : ''}`,
}

export const avatar = {
  catalog: (opts) => api.get('/api/avatar/catalog', opts),
  save: (universe, gender, style) => api.put('/api/me/avatar', { universe, gender, style }),
}

export const employee = {
  profile: (opts) => api.get('/api/me/profile', opts),
  assessments: (opts) => api.get('/api/assessments', opts),
  takeView: (assessmentId, opts) => api.get(`/api/assessments/${id(assessmentId)}/take`, opts),
  submit: (assessmentId, answers) => api.post(`/api/assessments/${id(assessmentId)}/submit`, { answers }),
  attempt: (attemptId, opts) => api.get(`/api/me/attempts/${id(attemptId)}`, opts),
}

export const characteristics = {
  list: (opts) => api.get('/api/characteristics', opts),
  create: (name) => api.post('/api/characteristics', { name }),
  rename: (charId, name) => api.put(`/api/characteristics/${id(charId)}`, { name }),
  remove: (charId) => api.del(`/api/characteristics/${id(charId)}`),
}

export const departments = {
  list: (includeInactive, opts) =>
    api.get('/api/departments', { ...opts, query: { includeInactive: includeInactive ? 1 : undefined } }),
  create: (name) => api.post('/api/departments', { name }),
  rename: (depId, name) => api.put(`/api/departments/${id(depId)}`, { name }),
  remove: (depId) => api.del(`/api/departments/${id(depId)}`),
  activate: (depId) => api.post(`/api/departments/${id(depId)}/activate`),
}

export const invitations = {
  list: (opts) => api.get('/api/invitations', opts),
  create: (expiresAt, maxRegistrations) => api.post('/api/invitations', { expiresAt, maxRegistrations }),
  update: (invId, patch) => api.patch(`/api/invitations/${id(invId)}`, patch),
  revoke: (invId) => api.post(`/api/invitations/${id(invId)}/revoke`),
  remove: (invId) => api.del(`/api/invitations/${id(invId)}`),
  registrations: (invId, opts) => api.get(`/api/invitations/${id(invId)}/registrations`, opts),
}

export const manage = {
  assessments: (opts) => api.get('/api/manage/assessments', opts),
  assessment: (aId, opts) => api.get(`/api/manage/assessments/${id(aId)}`, opts),
  createAssessment: (definition, idempotencyKey) =>
    api.post('/api/manage/assessments', definition, idempotencyKey ? { headers: { 'Idempotency-Key': idempotencyKey } } : undefined),
  updateAssessment: (aId, patch) => api.patch(`/api/manage/assessments/${id(aId)}`, patch),
  setVisibility: (aId, hidden) => api.put(`/api/manage/assessments/${id(aId)}/visibility`, { hidden }),
  setExpiry: (aId, expiresAt) => api.put(`/api/manage/assessments/${id(aId)}/expiry`, { expiresAt }),
  removeAssessment: (aId) => api.del(`/api/manage/assessments/${id(aId)}`),
}

export const admin = {
  users: (query, opts) => api.get('/api/admin/users', { ...opts, query }),
  setRole: (userId, role) => api.put(`/api/admin/users/${id(userId)}/role`, { role }),
  block: (userId) => api.post(`/api/admin/users/${id(userId)}/block`),
  unblock: (userId) => api.post(`/api/admin/users/${id(userId)}/unblock`),
  resetPassword: (userId, password) => api.post(`/api/admin/users/${id(userId)}/reset-password`, { password }),
}
