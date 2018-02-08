BEGIN;

ALTER TABLE instances ADD COLUMN command jsonb NOT NULL;

COMMIT;
