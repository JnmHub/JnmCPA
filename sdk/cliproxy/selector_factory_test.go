package cliproxy

import (
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestNewCoreAuthSelector_DefaultsToRoundRobin(t *testing.T) {
	t.Parallel()

	selector := newCoreAuthSelector(&config.Config{})
	if _, ok := selector.(*coreauth.RoundRobinSelector); !ok {
		t.Fatalf("newCoreAuthSelector() = %T, want *coreauth.RoundRobinSelector", selector)
	}
}

func TestNewCoreAuthSelector_SessionAffinityWrapsFallbackStrategy(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Routing.Strategy = "fill-first"
	cfg.Routing.SessionAffinity = true
	cfg.Routing.SessionAffinityTTL = "45m"

	selector := newCoreAuthSelector(cfg)
	sessionSelector, ok := selector.(*coreauth.SessionAffinitySelector)
	if !ok {
		t.Fatalf("newCoreAuthSelector() = %T, want *coreauth.SessionAffinitySelector", selector)
	}
	defer sessionSelector.Stop()
}

func TestSessionAffinityTTLFromConfig_InvalidFallsBackToDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Routing.SessionAffinityTTL = "not-a-duration"

	if got := sessionAffinityTTLFromConfig(cfg); got != defaultSessionAffinityTTL {
		t.Fatalf("sessionAffinityTTLFromConfig() = %s, want %s", got, defaultSessionAffinityTTL)
	}
}

func TestSelectorConfigKey_ChangesWithAffinitySettings(t *testing.T) {
	t.Parallel()

	base := &config.Config{}
	other := &config.Config{}
	other.Routing.SessionAffinity = true
	other.Routing.SessionAffinityTTL = "30m"

	if selectorConfigKey(base) == selectorConfigKey(other) {
		t.Fatalf("selectorConfigKey() should differ when session affinity changes")
	}

	if got := sessionAffinityTTLFromConfig(other); got != 30*time.Minute {
		t.Fatalf("sessionAffinityTTLFromConfig() = %s, want 30m", got)
	}
}
