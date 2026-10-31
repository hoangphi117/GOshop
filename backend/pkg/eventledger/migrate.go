package eventledger

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"goshop/pkg/eventledger/migrations"
)

// Migrate applies the embedded processed_events migrations on a dedicated
// migrations table, so one service database can host several embedded
// migration sets (one per version sequence) without version collisions.
// Any x-migrations-table parameter already present in connStr is replaced.
func Migrate(connStr, migrationsTable string) error {
	scopedConnStr, err := withMigrationsTable(connStr, migrationsTable)
	if err != nil {
		return fmt.Errorf("prepare event ledger migration connection: %w", err)
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load event ledger migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, scopedConnStr)
	if err != nil {
		return fmt.Errorf("init event ledger migrate instance: %w", err)
	}

	upErr := m.Up()
	closeSourceErr, closeDBErr := m.Close()

	if err := firstErr(upErr, closeSourceErr, closeDBErr); err != nil {
		return fmt.Errorf("apply event ledger migrations: %w", err)
	}
	return nil
}

// withMigrationsTable sets the x-migrations-table query parameter, replacing
// any value already present.
func withMigrationsTable(connStr, migrationsTable string) (string, error) {
	parsed, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("parse connection string: %w", err)
	}
	query := parsed.Query()
	query.Set("x-migrations-table", migrationsTable)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil && !errors.Is(e, migrate.ErrNoChange) {
			return e
		}
	}
	return nil
}
