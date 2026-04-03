package player

import (
	"testing"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

func TestPacketRateLimiterAllowHonorsConfiguredDelay(t *testing.T) {
	t.Parallel()

	limits := config.PacketRateLimitsConfig{
		Rules: map[config.PacketRateLimitKey]time.Duration{
			{Family: eonet.PacketFamily_Walk, Action: eonet.PacketAction_Player}: 300 * time.Millisecond,
		},
	}
	limiter := newPacketRateLimiter(limits)
	now := time.Unix(100, 0)

	if !limiter.Allow(now, eonet.PacketFamily_Walk, eonet.PacketAction_Player) {
		t.Fatal("first packet should pass")
	}
	if limiter.Allow(now.Add(250*time.Millisecond), eonet.PacketFamily_Walk, eonet.PacketAction_Player) {
		t.Fatal("packet inside configured limit should be ignored")
	}
	if !limiter.Allow(now.Add(300*time.Millisecond), eonet.PacketFamily_Walk, eonet.PacketAction_Player) {
		t.Fatal("packet at configured limit should pass")
	}
}

func TestPacketRateLimiterTracksPacketKeysIndependently(t *testing.T) {
	t.Parallel()

	limits := config.PacketRateLimitsConfig{
		Rules: map[config.PacketRateLimitKey]time.Duration{
			{Family: eonet.PacketFamily_Walk, Action: eonet.PacketAction_Player}: 300 * time.Millisecond,
		},
	}
	limiter := newPacketRateLimiter(limits)
	now := time.Unix(200, 0)

	if !limiter.Allow(now, eonet.PacketFamily_Walk, eonet.PacketAction_Player) {
		t.Fatal("walk packet should pass")
	}
	if !limiter.Allow(now.Add(10*time.Millisecond), eonet.PacketFamily_Talk, eonet.PacketAction_Report) {
		t.Fatal("unconfigured packet should not be throttled")
	}
	if limiter.Allow(now.Add(10*time.Millisecond), eonet.PacketFamily_Walk, eonet.PacketAction_Player) {
		t.Fatal("configured key should still be throttled independently")
	}
}
