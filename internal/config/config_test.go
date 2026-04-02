package config

import "testing"

func TestApplyEnvOverridesDatabase(t *testing.T) {
	t.Setenv("GEOSERV_DB_DRIVER", "mysql")
	t.Setenv("GEOSERV_DB_HOST", "db.internal")
	t.Setenv("GEOSERV_DB_PORT", "3307")
	t.Setenv("GEOSERV_DB_NAME", "prod_geoserv")
	t.Setenv("GEOSERV_DB_USERNAME", "app")
	t.Setenv("GEOSERV_DB_PASSWORD", "secret")

	cfg := &Config{
		Database: Database{
			Driver:   "sqlite",
			Host:     "127.0.0.1",
			Port:     "3306",
			Name:     "geoserv",
			Username: "geoserv",
			Password: "geoserv",
		},
	}

	applyEnvOverrides(cfg)

	if cfg.Database.Driver != "mysql" {
		t.Fatalf("expected driver override, got %q", cfg.Database.Driver)
	}
	if cfg.Database.Host != "db.internal" {
		t.Fatalf("expected host override, got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != "3307" {
		t.Fatalf("expected port override, got %q", cfg.Database.Port)
	}
	if cfg.Database.Name != "prod_geoserv" {
		t.Fatalf("expected name override, got %q", cfg.Database.Name)
	}
	if cfg.Database.Username != "app" {
		t.Fatalf("expected username override, got %q", cfg.Database.Username)
	}
	if cfg.Database.Password != "secret" {
		t.Fatalf("expected password override, got %q", cfg.Database.Password)
	}
}

func TestApplyEnvOverridesSkipsEmptyValues(t *testing.T) {
	t.Setenv("GEOSERV_DB_HOST", "")

	cfg := &Config{
		Database: Database{
			Host: "127.0.0.1",
		},
	}

	applyEnvOverrides(cfg)

	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("expected empty env var to be ignored, got %q", cfg.Database.Host)
	}
}
