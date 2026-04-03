package config

import (
	"fmt"
	"strings"
	"time"

	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

type PacketRateLimitEntry struct {
	Family string `yaml:"family"`
	Action string `yaml:"action"`
	Limit  int    `yaml:"limit"`
}

type PacketRateLimitKey struct {
	Family eonet.PacketFamily
	Action eonet.PacketAction
}

type PacketRateLimitsConfig struct {
	Limits map[string]int                       `yaml:"limits"`
	Rules  map[PacketRateLimitKey]time.Duration `yaml:"-"`
}

func (c *PacketRateLimitsConfig) Compile() error {
	c.Rules = make(map[PacketRateLimitKey]time.Duration, len(c.Limits))

	for key, limit := range c.Limits {
		family, action, err := parsePacketLimitKey(key)
		if err != nil {
			return err
		}
		if err := c.addRule(family, action, limit); err != nil {
			return err
		}
	}

	return nil
}

func (c PacketRateLimitsConfig) LimitFor(family eonet.PacketFamily, action eonet.PacketAction) (time.Duration, bool) {
	if len(c.Rules) == 0 {
		return 0, false
	}

	limit, ok := c.Rules[PacketRateLimitKey{Family: family, Action: action}]
	return limit, ok
}

func parsePacketFamily(value string) (eonet.PacketFamily, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for candidate := 0; candidate <= 255; candidate++ {
		family := eonet.PacketFamily(candidate)
		name, err := family.String()
		if err != nil {
			continue
		}
		if strings.ToLower(name) == normalized {
			return family, nil
		}
	}

	return 0, fmt.Errorf("unknown packet family %q", value)
}

func parsePacketAction(value string) (eonet.PacketAction, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for candidate := 0; candidate <= 255; candidate++ {
		action := eonet.PacketAction(candidate)
		name, err := action.String()
		if err != nil {
			continue
		}
		if strings.ToLower(name) == normalized {
			return action, nil
		}
	}

	return 0, fmt.Errorf("unknown packet action %q", value)
}

func parsePacketLimitKey(value string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid packet rate limit key %q: want Family.Action", value)
	}

	family := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if family == "" || action == "" {
		return "", "", fmt.Errorf("invalid packet rate limit key %q: want Family.Action", value)
	}

	return family, action, nil
}

func (c *PacketRateLimitsConfig) addRule(familyName, actionName string, limit int) error {
	family, err := parsePacketFamily(familyName)
	if err != nil {
		return err
	}

	action, err := parsePacketAction(actionName)
	if err != nil {
		return err
	}

	if limit < 0 {
		return fmt.Errorf("invalid packet rate limit for %s/%s: limit must be >= 0", familyName, actionName)
	}

	c.Rules[PacketRateLimitKey{Family: family, Action: action}] = time.Duration(limit) * time.Millisecond
	return nil
}
