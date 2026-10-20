-- The four protected default characteristics. `name` holds the ru name (used for
-- uniqueness); `name_i18n` carries the localized names shown in the UI.
-- The admin bootstrap stays in Go (ADMIN_LOGIN / ADMIN_PASSWORD, only when users is empty).
INSERT INTO characteristics (name, name_i18n, is_default) VALUES
  ('Критическое мышление', '{"ru":"Критическое мышление","kk":"Сыни ойлау"}', TRUE),
  ('Уверенность в себе',   '{"ru":"Уверенность в себе","kk":"Өзіне сенімділік"}', TRUE),
  ('Дисциплина',           '{"ru":"Дисциплина","kk":"Тәртіптілік"}', TRUE),
  ('Вовлечённость',        '{"ru":"Вовлечённость","kk":"Белсенді қатысу"}', TRUE);
