-- BI Profily baseline schema (redesign). Applied once, tracked in schema_migrations.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE departments (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
  active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_departments_active_name ON departments (lower(name)) WHERE active;

CREATE TABLE invitations (
  id                  BIGSERIAL PRIMARY KEY,
  code                TEXT NOT NULL UNIQUE,
  created_by          BIGINT NOT NULL,
  expires_at          TIMESTAMPTZ,
  max_registrations   INTEGER NOT NULL CHECK (max_registrations BETWEEN 1 AND 10000),
  registrations_count INTEGER NOT NULL DEFAULT 0 CHECK (registrations_count >= 0),
  revoked_at          TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT invitations_count_le_max CHECK (registrations_count <= max_registrations)
);

CREATE TABLE users (
  id              BIGSERIAL PRIMARY KEY,
  login           CITEXT NOT NULL UNIQUE CHECK (login ~ '^[A-Za-z0-9._-]{3,64}$'),
  pass_hash       TEXT NOT NULL,
  alg_ver         SMALLINT NOT NULL DEFAULT 1,
  full_name       TEXT NOT NULL CHECK (length(btrim(full_name)) BETWEEN 1 AND 160),
  job_title       TEXT NOT NULL DEFAULT '' CHECK (length(job_title) <= 160),
  department_id   BIGINT REFERENCES departments(id) ON DELETE RESTRICT,
  invitation_id   BIGINT REFERENCES invitations(id) ON DELETE RESTRICT,
  role            TEXT NOT NULL CHECK (role IN ('admin','manager','employee')),
  status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked')),
  avatar_universe TEXT,
  avatar_gender   TEXT,
  avatar_style    TEXT,
  avatar_set_at   TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT users_avatar_all_or_none CHECK (
    (avatar_universe IS NULL AND avatar_gender IS NULL AND avatar_style IS NULL AND avatar_set_at IS NULL) OR
    (avatar_universe IS NOT NULL AND avatar_gender IS NOT NULL AND avatar_style IS NOT NULL AND avatar_set_at IS NOT NULL))
);
ALTER TABLE invitations ADD CONSTRAINT invitations_created_by_fk FOREIGN KEY (created_by) REFERENCES users(id);
CREATE INDEX idx_users_invitation ON users (invitation_id);
CREATE INDEX idx_users_department ON users (department_id);

CREATE TABLE characteristics (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
  name_i18n  JSONB,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_characteristics_name ON characteristics (lower(name));

CREATE TABLE assessments (
  id          BIGSERIAL PRIMARY KEY,
  title       TEXT NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
  description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 4000),
  hidden      BOOLEAN NOT NULL DEFAULT FALSE,
  expires_at  TIMESTAMPTZ,
  created_by  BIGINT NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

CREATE TABLE questions (
  id            BIGSERIAL PRIMARY KEY,
  assessment_id BIGINT NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
  position      INTEGER NOT NULL CHECK (position >= 0),
  text          TEXT NOT NULL CHECK (length(btrim(text)) BETWEEN 1 AND 1000),
  type          TEXT NOT NULL CHECK (type IN ('single','multiple','rating')),
  UNIQUE (assessment_id, position)
);

CREATE TABLE options (
  id          BIGSERIAL PRIMARY KEY,
  question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL CHECK (position >= 0),
  text        TEXT NOT NULL CHECK (length(btrim(text)) BETWEEN 1 AND 500),
  UNIQUE (question_id, position),
  UNIQUE (id, question_id)
);

CREATE TABLE option_effects (
  option_id         BIGINT NOT NULL REFERENCES options(id) ON DELETE CASCADE,
  characteristic_id BIGINT NOT NULL REFERENCES characteristics(id) ON DELETE RESTRICT,
  level             TEXT NOT NULL CHECK (level IN ('low','moderate','high')),
  direction         SMALLINT NOT NULL CHECK (direction IN (-1, 1)),
  PRIMARY KEY (option_id, characteristic_id)
);
CREATE INDEX idx_option_effects_char ON option_effects (characteristic_id);

-- Question structure is immutable after creation (defense in depth: the API has no update paths).
CREATE FUNCTION reject_definition_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'assessment definitions are immutable (% on %)', TG_OP, TG_TABLE_NAME;
END $$;
CREATE TRIGGER questions_immutable BEFORE UPDATE ON questions
  FOR EACH ROW EXECUTE FUNCTION reject_definition_update();
CREATE TRIGGER options_immutable BEFORE UPDATE ON options
  FOR EACH ROW EXECUTE FUNCTION reject_definition_update();
CREATE TRIGGER option_effects_immutable BEFORE UPDATE ON option_effects
  FOR EACH ROW EXECUTE FUNCTION reject_definition_update();

CREATE TABLE attempts (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id),
  assessment_id BIGINT NOT NULL REFERENCES assessments(id) ON DELETE RESTRICT,
  completed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, assessment_id)
);
CREATE INDEX idx_attempts_user_time ON attempts (user_id, completed_at);

-- Normalized answers. rating_value dropped (plan sec. 17 #4): the chosen option row is the scale point.
CREATE TABLE attempt_answers (
  attempt_id  BIGINT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE RESTRICT,
  option_id   BIGINT NOT NULL,
  PRIMARY KEY (attempt_id, option_id),
  FOREIGN KEY (option_id, question_id) REFERENCES options(id, question_id) ON DELETE RESTRICT
);
CREATE INDEX idx_attempt_answers_question ON attempt_answers (question_id);

CREATE TABLE attempt_scores (
  attempt_id        BIGINT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  characteristic_id BIGINT NOT NULL REFERENCES characteristics(id) ON DELETE RESTRICT,
  score             NUMERIC(5,2) NOT NULL CHECK (score BETWEEN 0 AND 100),
  PRIMARY KEY (attempt_id, characteristic_id)
);

-- Audit log: INSERT only from the service layer. Passwords and tokens never go here.
CREATE TABLE audit_log (
  id          BIGSERIAL PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor_id    BIGINT REFERENCES users(id),
  actor_role  TEXT,
  action      TEXT NOT NULL,
  entity_type TEXT,
  entity_id   TEXT,
  ip          TEXT,
  result      TEXT NOT NULL
);
CREATE INDEX idx_audit_entity ON audit_log (entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_actor ON audit_log (actor_id, occurred_at DESC);
