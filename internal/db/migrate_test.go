package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/avdoseferovic/geoserv/internal/config"
)

func TestMigrationNames(t *testing.T) {
	t.Parallel()

	names, err := migrationNames("sqlite")
	if err != nil {
		t.Fatalf("migrationNames returned error: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 sqlite migrations, got %d", len(names))
	}
}

func TestMySQLMigrationsUseCompatibleIndexableTypes(t *testing.T) {
	t.Parallel()

	data, err := migrationFiles.ReadFile("migrations/mysql/000001_init.up.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	sql := string(data)
	forbidden := []string{
		"`name` TEXT NOT NULL UNIQUE",
		"CREATE INDEX IF NOT EXISTS",
	}

	for _, fragment := range forbidden {
		if strings.Contains(sql, fragment) {
			t.Fatalf("mysql migration contains unsupported fragment %q", fragment)
		}
	}
}

func TestDatabaseMigrateSQLite(t *testing.T) {
	t.Parallel()

	cfg := config.Database{
		Driver: "sqlite",
		Name:   filepath.Join(t.TempDir(), "geoserv-test"),
	}

	database, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if err := database.Migrate(); err != nil {
		t.Fatalf("first Migrate returned error: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("second Migrate returned error: %v", err)
	}

	var count int
	row := database.QueryRow(t.Context(), `SELECT COUNT(*) FROM schema_migrations`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scanning schema_migrations returned error: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 applied migration, got %d", count)
	}

	row = database.QueryRow(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'accounts'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scanning sqlite_master returned error: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected accounts table to exist, got count %d", count)
	}
}
