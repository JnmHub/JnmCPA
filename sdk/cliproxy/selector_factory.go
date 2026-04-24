package cliproxy

import (
	"strings"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
)

const defaultSessionAffinityTTL = time.Hour

func selectorConfigKey(cfg *config.Config) string {
	if cfg == nil {
		return "round-robin|false|1h0m0s"
	}
	strategy := normalizeRoutingStrategy(cfg.Routing.Strategy)
	enabled := routingSessionAffinityEnabled(cfg)
	ttl := sessionAffinityTTLFromConfig(cfg)
	return strategy + "|" + boolString(enabled) + "|" + ttl.String()
}

func newCoreAuthSelector(cfg *config.Config) coreauth.Selector {
	fallback := buildBaseSelector(cfg)
	if !routingSessionAffinityEnabled(cfg) {
		return fallback
	}
	return coreauth.NewSessionAffinitySelectorWithConfig(coreauth.SessionAffinityConfig{
		Fallback: fallback,
		TTL:      sessionAffinityTTLFromConfig(cfg),
	})
}

func buildBaseSelector(cfg *config.Config) coreauth.Selector {
	switch normalizeRoutingStrategy(routingStrategy(cfg)) {
	case "fill-first":
		return &coreauth.FillFirstSelector{}
	default:
		return &coreauth.RoundRobinSelector{}
	}
}

func routingStrategy(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.Routing.Strategy
}

func normalizeRoutingStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "fill-first", "fillfirst", "ff":
		return "fill-first"
	default:
		return "round-robin"
	}
}

func routingSessionAffinityEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Routing.SessionAffinity || cfg.Routing.ClaudeCodeSessionAffinity
}

func sessionAffinityTTLFromConfig(cfg *config.Config) time.Duration {
	if cfg == nil {
		return defaultSessionAffinityTTL
	}
	raw := strings.TrimSpace(cfg.Routing.SessionAffinityTTL)
	if raw == "" {
		return defaultSessionAffinityTTL
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil {
		log.Warnf("invalid routing.session-affinity-ttl %q; using default %s", raw, defaultSessionAffinityTTL)
		return defaultSessionAffinityTTL
	}
	if ttl <= 0 {
		return defaultSessionAffinityTTL
	}
	return ttl
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
