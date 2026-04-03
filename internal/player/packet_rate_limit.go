package player

import (
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	eonet "github.com/ethanmoffat/eolib-go/v3/protocol/net"
)

type packetRateLimiter struct {
	limits   config.PacketRateLimitsConfig
	lastSeen map[config.PacketRateLimitKey]time.Time
}

func newPacketRateLimiter(limits config.PacketRateLimitsConfig) *packetRateLimiter {
	return &packetRateLimiter{
		limits:   limits,
		lastSeen: make(map[config.PacketRateLimitKey]time.Time, len(limits.Rules)),
	}
}

func (l *packetRateLimiter) Allow(now time.Time, family eonet.PacketFamily, action eonet.PacketAction) bool {
	limit, ok := l.limits.LimitFor(family, action)
	if !ok || limit <= 0 {
		return true
	}

	key := config.PacketRateLimitKey{Family: family, Action: action}
	lastSeen, ok := l.lastSeen[key]
	if ok && now.Sub(lastSeen) < limit {
		return false
	}

	l.lastSeen[key] = now
	return true
}
