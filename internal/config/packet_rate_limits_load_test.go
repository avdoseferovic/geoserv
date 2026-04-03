package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

func TestLoadPacketRateLimitsConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, "server.yaml", "server:\n  host: \"127.0.0.1\"\n")
	writeConfigFile(t, dir, "gameplay.yaml", "world:\n  max_map: 0\n")
	writeConfigFile(t, dir, "rate_limits.yaml", "limits:\n  Walk.Player: 180\n  Attack.Use: 350\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	walkLimit, ok := cfg.PacketRateLimits.LimitFor(eonet.PacketFamily_Walk, eonet.PacketAction_Player)
	if !ok {
		t.Fatal("expected walk/player rate limit to be loaded")
	}
	if walkLimit != 180*time.Millisecond {
		t.Fatalf("walk/player limit = %v, want 180ms", walkLimit)
	}

	attackLimit, ok := cfg.PacketRateLimits.LimitFor(eonet.PacketFamily_Attack, eonet.PacketAction_Use)
	if !ok {
		t.Fatal("expected attack/use rate limit to be loaded")
	}
	if attackLimit != 350*time.Millisecond {
		t.Fatalf("attack/use limit = %v, want 350ms", attackLimit)
	}
}

func TestLoadPacketRateLimitsConfigRejectsUnknownPacketName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfigFile(t, dir, "server.yaml", "server:\n  host: \"127.0.0.1\"\n")
	writeConfigFile(t, dir, "gameplay.yaml", "world:\n  max_map: 0\n")
	writeConfigFile(t, dir, "rate_limits.yaml", "limits:\n  Nope.Player: 300\n")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid packet family error")
	}
}

func writeConfigFile(t *testing.T, dir, name, contents string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
