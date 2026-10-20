-- Legacy-schema guard. The new baseline (001+) cannot coexist with the pre-redesign
-- schema: CREATE TABLE would silently keep the old shapes. Abort instead.
DO $$
BEGIN
  IF to_regclass('public.test_versions') IS NOT NULL
     OR to_regclass('public.units') IS NOT NULL
     OR to_regclass('public.results') IS NOT NULL THEN
    RAISE EXCEPTION 'legacy schema detected: reset volume (docker compose down -v) or run with MIGRATE_RESET=1 in dev';
  END IF;
END $$;
