BEGIN;

CREATE TABLE specs (
	id uuid NOT NULL PRIMARY KEY,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	assignment_name text NOT NULL,
	data jsonb NOT NULL
);


CREATE TABLE instances (
	id uuid NOT NULL PRIMARY KEY,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL,
	spec_id uuid REFERENCES specs(id),
	image_id text NOT NULL,
	container_id text NOT NULL,
	expires_at timestamptz,
	active boolean NOT NULL,
	cleaned boolean NOT NULL
);


COMMIT;
