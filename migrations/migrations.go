package migrations

import (
	"database/sql"

	"github.com/mattes/migrate"
	"github.com/mattes/migrate/database/postgres"
	"github.com/mattes/migrate/source/go-bindata"
)

//go:generate go-bindata -pkg migrations -ignore=\.(go|json|md)$ .

// Up brings the database up to date to the latest migration.
func Up(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	return ignoreNoChange(m.Up())
}

// Down brings the database down by applying the down migrations.
func Down(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	return ignoreNoChange(m.Down())
}

// Reset resets the database by bringing the database down and up again.
func Reset(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	if err := m.Drop(); err != nil {
		return err
	}
	return ignoreNoChange(m.Up())
}

func newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	resource := bindata.Resource(AssetNames(),
		func(name string) ([]byte, error) {
			return Asset(name)
		},
	)
	source, err := bindata.WithInstance(resource)
	if err != nil {
		return nil, err
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	return migrate.NewWithInstance("go-bindata", source, "postgres", driver)
}

func ignoreNoChange(err error) error {
	if err == migrate.ErrNoChange {
		return nil
	}
	return err
}
