package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server           Server                 `yaml:"server"`
	Database         Database               `yaml:"database"`
	SLN              SLN                    `yaml:"sln"`
	Account          Account                `yaml:"account"`
	PacketRateLimits PacketRateLimitsConfig `yaml:"-"`
	SMTP             SMTP                   `yaml:"smtp"`
	NewCharacter     NewCharacter           `yaml:"new_character"`
	Jail             Jail                   `yaml:"jail"`
	Rescue           Rescue                 `yaml:"rescue"`
	World            World                  `yaml:"world"`
	Bard             Bard                   `yaml:"bard"`
	Combat           Combat                 `yaml:"combat"`
	Map              Map                    `yaml:"map"`
	Character        Character              `yaml:"character"`
	NPCs             NPCs                   `yaml:"npcs"`
	Bank             Bank                   `yaml:"bank"`
	Limits           Limits                 `yaml:"limits"`
	Board            Board                  `yaml:"board"`
	Chest            Chest                  `yaml:"chest"`
	Jukebox          Jukebox                `yaml:"jukebox"`
	Barber           Barber                 `yaml:"barber"`
	Guild            Guild                  `yaml:"guild"`
	Marriage         Marriage               `yaml:"marriage"`
	Evacuate         Evacuate               `yaml:"evacuate"`
	Items            Items                  `yaml:"items"`
	AutoPickup       AutoPickup             `yaml:"auto_pickup"`
	Content          Content                `yaml:"content"`
	Arenas           Arenas                 `yaml:"arenas"`
}

// Load reads config from a directory containing server.yaml, gameplay.yaml,
// and rate_limits.yaml. Each file can have a .local.yaml override.
func Load(dir string) (*Config, error) {
	var cfg Config

	files := []string{"server.yaml", "gameplay.yaml"}
	for _, name := range files {
		if err := loadYAML(filepath.Join(dir, name), &cfg); err != nil {
			return nil, err
		}

		// Apply local overrides (e.g. server.local.yaml)
		localName := name[:len(name)-len(".yaml")] + ".local.yaml"
		localPath := filepath.Join(dir, localName)
		if _, err := os.Stat(localPath); err == nil {
			if err := loadYAML(localPath, &cfg); err != nil {
				return nil, err
			}
		}
	}

	if err := loadPacketRateLimits(filepath.Join(dir, "rate_limits.yaml"), &cfg); err != nil {
		return nil, err
	}
	localPacketRateLimits := filepath.Join(dir, "rate_limits.local.yaml")
	if _, err := os.Stat(localPacketRateLimits); err == nil {
		if err := loadPacketRateLimits(localPacketRateLimits, &cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(&cfg)

	return &cfg, nil
}

func loadYAML(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}
	return nil
}

func loadPacketRateLimits(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", path, err)
	}

	var limits PacketRateLimitsConfig
	if err := yaml.Unmarshal(data, &limits); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}
	if err := limits.Compile(); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}

	cfg.PacketRateLimits = limits
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	overrideString(&cfg.Database.Driver, "GEOSERV_DB_DRIVER")
	overrideString(&cfg.Database.Host, "GEOSERV_DB_HOST")
	overrideString(&cfg.Database.Port, "GEOSERV_DB_PORT")
	overrideString(&cfg.Database.Name, "GEOSERV_DB_NAME")
	overrideString(&cfg.Database.Username, "GEOSERV_DB_USERNAME")
	overrideString(&cfg.Database.Password, "GEOSERV_DB_PASSWORD")
}

func overrideString(target *string, envKey string) {
	if value, ok := os.LookupEnv(envKey); ok && value != "" {
		*target = value
	}
}
