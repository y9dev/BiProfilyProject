// Frozen API contract, frontend side (AC-13). Loads the same golden fixtures as
// backend/internal/httpapi/dto/golden_test.go and checks that each has exactly
// the shape the frontend parsers (src/api/endpoints.js, Phase 2) rely on.
// A contract change updates the Go DTO, the golden file, this schema and the
// parser together.
//
// Run: node --test frontend/src/api/contract.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const goldenDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', 'contracts', 'golden');

// ---- tiny schema language ----
// 'string' | 'number' | 'integer' | 'boolean' | 'iso' (RFC 3339 timestamp) | 'date' (YYYY-MM-DD)
// nullable(s), optional(s) (key may be absent), oneOf(...values), [s] (array of s), { key: s } (exact keys)
const nullable = (s) => ({ kind: 'nullable', s });
const optional = (s) => ({ kind: 'optional', s });
const oneOf = (...values) => ({ kind: 'oneOf', values });
const ISO = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const DATE = /^\d{4}-\d{2}-\d{2}$/;

function check(value, schema, path) {
  if (typeof schema === 'string') {
    const ok = {
      string: () => typeof value === 'string',
      number: () => typeof value === 'number' && Number.isFinite(value),
      integer: () => Number.isInteger(value),
      boolean: () => typeof value === 'boolean',
      iso: () => typeof value === 'string' && ISO.test(value),
      date: () => typeof value === 'string' && DATE.test(value),
    }[schema];
    assert.ok(ok, `${path}: unknown schema type ${schema}`);
    assert.ok(ok(), `${path}: expected ${schema}, got ${JSON.stringify(value)}`);
    return;
  }
  if (Array.isArray(schema)) {
    assert.ok(Array.isArray(value), `${path}: expected array, got ${JSON.stringify(value)}`);
    value.forEach((v, i) => check(v, schema[0], `${path}[${i}]`));
    return;
  }
  switch (schema.kind) {
    case 'nullable':
      if (value !== null) check(value, schema.s, path);
      return;
    case 'optional':
      check(value, schema.s, path);
      return;
    case 'oneOf':
      assert.ok(schema.values.includes(value), `${path}: ${JSON.stringify(value)} not in ${schema.values.join('|')}`);
      return;
    default: {
      assert.ok(value !== null && typeof value === 'object' && !Array.isArray(value), `${path}: expected object`);
      for (const key of Object.keys(value)) {
        assert.ok(key in schema, `${path}.${key}: unexpected key (contract drift)`);
      }
      for (const [key, s] of Object.entries(schema)) {
        if (!(key in value)) {
          assert.ok(s?.kind === 'optional', `${path}.${key}: missing required key`);
          continue;
        }
        check(value[key], s, `${path}.${key}`);
      }
    }
  }
}

// ---- shared shapes ----
const i18n = nullable({ ru: 'string', kk: 'string' });
const departmentRef = { id: 'integer', name: 'string' };
const avatar = { universe: 'string', gender: 'string', style: 'string' };
const score = { characteristicId: 'integer', name: 'string', nameI18n: i18n, score: 'number' };
const effect = { characteristicId: 'integer', level: oneOf('low', 'moderate', 'high'), direction: oneOf(-1, 1) };
const questionType = oneOf('single', 'multiple', 'rating');
const role = oneOf('admin', 'manager', 'employee');
const invitation = {
  id: 'integer',
  code: 'string',
  url: 'string',
  expiresAt: nullable('iso'),
  maxRegistrations: 'integer',
  registrationsCount: 'integer',
  status: oneOf('active', 'expired', 'full', 'revoked'),
  createdAt: 'iso',
  createdBy: { id: 'integer', fullName: 'string' },
};
const assessmentStatus = oneOf('active', 'hidden', 'expired');
const registerOptions = { valid: 'boolean', reason: optional('string'), departments: optional([departmentRef]) };
const error = { error: 'string', message: optional('string'), errors: optional([{ path: 'string', code: 'string' }]) };

// ---- dashboard / analytics shapes ----
const day = 'date'; // YYYY-MM-DD (Asia/Almaty calendar day)
const userRef = { id: 'integer', fullName: 'string' };
const aDept = nullable({ id: 'integer', name: 'string', active: 'boolean' });
const charRef = { id: 'integer', name: 'string', nameI18n: i18n, core: 'boolean' };
const period = { from: day, to: day, granularity: oneOf('day', 'week', 'month'), timezone: 'string' };
const valuePoint = { value: nullable('number'), employees: 'integer', attempts: 'integer', insufficient: 'boolean' };
const trend = {
  period,
  minSample: 'integer',
  points: [day],
  series: [{ characteristic: charRef, values: [valuePoint] }],
  summary: { employees: 'integer', measuredEmployees: 'integer', attemptsInPeriod: 'integer' },
};
const testCompletion = {
  id: 'integer',
  title: 'string',
  completed: 'integer',
  employees: 'integer',
  rate: nullable('number'),
};
const reason = {
  code: oneOf('drop', 'inactive', 'no_first_test'),
  characteristicId: optional('integer'),
  delta: optional('number'),
  at: optional(day),
  days: optional('integer'),
};
const changePoint = { score: 'number', at: 'iso' };

// ---- one schema per golden file ----
const schemas = {
  'me.json': {
    id: 'integer',
    login: 'string',
    fullName: 'string',
    jobTitle: 'string',
    department: nullable(departmentRef),
    role,
    avatar: nullable(avatar),
    profileComplete: 'boolean',
  },
  'register_options.json': registerOptions,
  'register_options_invalid.json': registerOptions,
  'avatar_catalog.json': { universes: ['string'], genders: ['string'], styles: ['string'] },
  'profile.json': {
    characteristics: [{ id: 'integer', name: 'string', nameI18n: i18n, mean: 'number', count: 'integer' }],
    timeline: [{ attemptId: 'integer', assessmentTitle: 'string', completedAt: 'iso', scores: [score] }],
    stats: { completed: 'integer', available: 'integer' },
  },
  'assessments_list.json': [
    {
      id: 'integer',
      title: 'string',
      description: 'string',
      expiresAt: nullable('iso'),
      questionCount: 'integer',
      completed: 'boolean',
      attemptId: optional('integer'),
    },
  ],
  'take_view.json': {
    id: 'integer',
    title: 'string',
    description: 'string',
    questions: [
      {
        id: 'integer',
        position: 'integer',
        text: 'string',
        type: questionType,
        options: [{ id: 'integer', position: 'integer', text: 'string' }],
      },
    ],
  },
  'submit_request.json': { answers: [{ questionId: 'integer', optionIds: ['integer'] }] },
  'submit_result.json': { attemptId: 'integer', scores: [score] },
  'attempt_result.json': {
    attemptId: 'integer',
    assessmentId: 'integer',
    assessmentTitle: 'string',
    completedAt: 'iso',
    scores: [score],
  },
  'characteristics.json': [
    { id: 'integer', name: 'string', nameI18n: i18n, isDefault: 'boolean', inUse: 'boolean' },
  ],
  'departments.json': [{ id: 'integer', name: 'string', active: 'boolean', userCount: 'integer' }],
  'department_delete.json': { deactivated: 'boolean' },
  'invitation.json': invitation,
  'invitations_list.json': [invitation],
  'registrations.json': [{ fullName: 'string', jobTitle: 'string', department: nullable(departmentRef) }],
  'manage_assessments_list.json': [
    {
      id: 'integer',
      title: 'string',
      description: 'string',
      hidden: 'boolean',
      expiresAt: nullable('iso'),
      status: assessmentStatus,
      questionCount: 'integer',
      attemptCount: 'integer',
      createdAt: 'iso',
      updatedAt: 'iso',
    },
  ],
  'manage_assessment.json': {
    id: 'integer',
    title: 'string',
    description: 'string',
    hidden: 'boolean',
    expiresAt: nullable('iso'),
    status: assessmentStatus,
    attemptCount: 'integer',
    createdAt: 'iso',
    updatedAt: 'iso',
    questions: [
      {
        id: 'integer',
        position: 'integer',
        text: 'string',
        type: questionType,
        options: [{ id: 'integer', position: 'integer', text: 'string', effects: [effect] }],
      },
    ],
  },
  'create_assessment_request.json': {
    title: 'string',
    description: 'string',
    expiresAt: nullable('iso'),
    hidden: 'boolean',
    questions: [{ text: 'string', type: questionType, options: [{ text: 'string', effects: [effect] }] }],
  },
  'create_assessment_result.json': {
    id: 'integer',
    warnings: [{ path: 'string', code: 'string', characteristicId: 'integer' }],
  },
  'admin_users.json': {
    items: [
      {
        id: 'integer',
        login: 'string',
        fullName: 'string',
        jobTitle: 'string',
        department: nullable(departmentRef),
        role,
        status: oneOf('active', 'blocked'),
        createdAt: 'iso',
      },
    ],
    total: 'integer',
  },
  'error_validation.json': error,
  'error.json': error,
  'dashboard_trend.json': trend,
  'analytics_trend.json': {
    ...trend,
    defaultIds: ['integer'],
    characteristics: [{ ...charRef, employees: 'integer', attempts: 'integer', eligible: 'boolean' }],
  },
  'team_status.json': {
    asOf: day,
    inactiveDays: 'integer',
    dropThreshold: 'integer',
    kpis: {
      employees: 'integer',
      participated: 'integer',
      noAttempts: 'integer',
      activeRecently: 'integer',
      inactive: 'integer',
      participationRate: nullable('number'),
    },
    tests: [testCompletion],
    attention: {
      total: 'integer',
      counts: { noFirstTest: 'integer', inactive: 'integer', drop: 'integer' },
      items: [{ user: userRef, department: aDept, lastAttemptAt: nullable('iso'), reasons: [reason] }],
    },
  },
  'analytics_participation.json': {
    period,
    company: { employees: 'integer', participated: 'integer', attempts: 'integer', rate: nullable('number') },
    departments: [
      {
        department: aDept,
        employees: 'integer',
        participated: 'integer',
        attempts: 'integer',
        rate: nullable('number'),
        insufficient: 'boolean',
      },
    ],
    tests: [testCompletion],
  },
  'analytics_departments.json': {
    period,
    points: [day],
    blocks: [
      {
        characteristic: charRef,
        company: { values: [valuePoint] },
        departments: [
          {
            department: aDept,
            employees: 'integer',
            attemptsInPeriod: 'integer',
            values: [valuePoint],
            current: { ...valuePoint, vsCompany: nullable('number') },
          },
        ],
      },
    ],
  },
  'analytics_employees.json': {
    total: 'integer',
    items: [
      {
        id: 'integer',
        fullName: 'string',
        jobTitle: 'string',
        department: aDept,
        attempts: 'integer',
        lastAttemptAt: nullable('iso'),
        dataState: oneOf('ok', 'no_data', 'stale'),
      },
    ],
  },
  'analytics_employee.json': {
    user: {
      id: 'integer',
      fullName: 'string',
      jobTitle: 'string',
      department: aDept,
      status: oneOf('active', 'blocked'),
      lastAttemptAt: nullable('iso'),
    },
    asOf: day,
    current: [
      { characteristic: charRef, score: 'number', measuredAt: 'iso', company: valuePoint, department: valuePoint },
    ],
    history: [{ attemptId: 'integer', assessmentTitle: 'string', completedAt: 'iso', scores: [score] }],
  },
  'analytics_ranking.json': {
    asOf: day,
    minSample: 'integer',
    blocks: [
      {
        characteristic: charRef,
        insufficient: 'boolean',
        employees: {
          total: 'integer',
          items: [{ rank: 'integer', user: userRef, department: aDept, score: 'number', measuredAt: 'iso' }],
        },
        departments: [
          { rank: 'integer', department: aDept, value: 'number', employees: 'integer', attempts: 'integer' },
        ],
      },
    ],
  },
  'analytics_changes.json': {
    threshold: 'integer',
    total: 'integer',
    items: [
      {
        user: userRef,
        department: aDept,
        characteristicId: 'integer',
        from: changePoint,
        to: changePoint,
        delta: 'number',
      },
    ],
  },
};

const load = (name) => JSON.parse(readFileSync(join(goldenDir, name), 'utf8'));

test('every golden fixture has a frontend schema and vice versa', () => {
  const files = readdirSync(goldenDir).filter((f) => f.endsWith('.json')).sort();
  assert.deepEqual(files, Object.keys(schemas).sort());
});

for (const [name, schema] of Object.entries(schemas)) {
  test(`contract: ${name}`, () => {
    check(load(name), schema, name.replace(/\.json$/, ''));
  });
}

test('take view carries no scoring data', () => {
  const raw = readFileSync(join(goldenDir, 'take_view.json'), 'utf8');
  for (const key of ['effects', 'level', 'direction', 'characteristicId']) {
    assert.ok(!raw.includes(`"${key}"`), `take_view leaks ${key}`);
  }
});

test('avatar catalog matches the agreed 5 x 2 x 4 ids', () => {
  assert.deepEqual(load('avatar_catalog.json'), {
    universes: ['movies', 'anime', 'animals', 'sports', 'fantasy'],
    genders: ['male', 'female'],
    styles: ['vibrant', 'formal', 'calm', 'tech'],
  });
});

test('validation errors use the {error, errors:[{path, code}]} envelope', () => {
  const body = load('error_validation.json');
  assert.equal(body.error, 'validation');
  assert.ok(body.errors.length > 0);
});

// Every object carrying both `core` and `nameI18n`: the frontend heuristic
// (nameI18n != null, features/employee/trends.js) must agree with the server's is_default.
test('core === (nameI18n !== null) in every golden', () => {
  let seen = 0;
  const walk = (v, path) => {
    if (Array.isArray(v)) return v.forEach((x, i) => walk(x, `${path}[${i}]`));
    if (v === null || typeof v !== 'object') return;
    if ('core' in v && 'nameI18n' in v) {
      seen++;
      assert.equal(v.core, v.nameI18n !== null, `${path}: core=${v.core} but nameI18n=${JSON.stringify(v.nameI18n)}`);
    }
    for (const [k, x] of Object.entries(v)) walk(x, `${path}.${k}`);
  };
  for (const f of readdirSync(goldenDir).filter((f) => f.endsWith('.json'))) walk(load(f), f);
  assert.ok(seen > 0, 'no characteristic refs found');
});

test('analytics goldens cover the required fixture variants', () => {
  const raw = (name) => readFileSync(join(goldenDir, name), 'utf8');
  const points = (name) => load(name).series.flatMap((s) => s.values);
  // null gap; insufficient with a value (admin) and with null (manager)
  assert.ok(points('dashboard_trend.json').some((p) => p.employees === 0 && p.value === null));
  assert.ok(points('dashboard_trend.json').some((p) => p.employees > 0 && p.insufficient && p.value === null));
  assert.ok(points('analytics_trend.json').some((p) => p.insufficient && p.value !== null));
  // every reason variant, department null, lastAttemptAt null, inactive department
  const items = load('team_status.json').attention.items;
  const codes = new Set(items.flatMap((i) => i.reasons.map((r) => r.code)));
  assert.deepEqual([...codes].sort(), ['drop', 'inactive', 'no_first_test']);
  assert.ok(items.some((i) => i.department === null && i.lastAttemptAt === null));
  assert.ok(items.some((i) => i.department?.active === false));
  // an insufficient ranking block without employee items
  assert.ok(load('analytics_ranking.json').blocks.some((b) => b.insufficient && b.employees.items.length === 0));
  for (const name of ['analytics_participation.json', 'analytics_departments.json', 'analytics_employees.json', 'analytics_changes.json']) {
    assert.ok(raw(name).includes('"department": null'), `${name}: no department:null fixture`);
  }
});
