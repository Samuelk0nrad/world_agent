package policy

import (
	"fmt"
	"strconv"

	"github.com/spf13/viper"
	"worldagent/agent-backend/internal/config"
)

const (
	CapabilityWebSearch     = "web-search"
	CapabilityEmail         = "email"
	CapabilityMobileSensors = "mobile-sensors"
	CapabilityScreenCapture = "screen-capture"
	CapabilityAudioCapture  = "audio-capture"
)

type DisabledError struct {
	Capability string
	Tool       string
}

func (e DisabledError) Error() string {
	if e.Tool == "" {
		return fmt.Sprintf("capability %q is disabled by policy", e.Capability)
	}
	return fmt.Sprintf("tool %q is disabled by policy capability %q", e.Tool, e.Capability)
}

type Gate struct {
	allowed map[string]bool
}

func NewDefaultGate() Gate {
	return Gate{
		allowed: map[string]bool{
			CapabilityWebSearch:     true,
			CapabilityEmail:         false,
			CapabilityMobileSensors: false,
			CapabilityScreenCapture: false,
			CapabilityAudioCapture:  false,
		},
	}
}

func LoadFromEnv() Gate {
	return LoadFromViper(config.MustViper())
}

func LoadFromViper(cfg *viper.Viper) Gate {
	gate := NewDefaultGate()
	gate = gate.withViperOverride(cfg, "AGENT_CAPABILITY_WEB_SEARCH", CapabilityWebSearch)
	gate = gate.withViperOverride(cfg, "AGENT_CAPABILITY_EMAIL", CapabilityEmail)
	gate = gate.withViperOverride(cfg, "AGENT_CAPABILITY_MOBILE_SENSORS", CapabilityMobileSensors)
	gate = gate.withViperOverride(cfg, "AGENT_CAPABILITY_SCREEN_CAPTURE", CapabilityScreenCapture)
	gate = gate.withViperOverride(cfg, "AGENT_CAPABILITY_AUDIO_CAPTURE", CapabilityAudioCapture)
	return gate
}

func (g Gate) WithCapability(capability string, enabled bool) Gate {
	allowed := g.resolveAllowed()
	next := make(map[string]bool, len(allowed))
	for key, value := range allowed {
		next[key] = value
	}
	next[capability] = enabled
	return Gate{allowed: next}
}

func (g Gate) IsAllowed(capability string) bool {
	enabled, ok := g.resolveAllowed()[capability]
	return ok && enabled
}

func (g Gate) RequireTool(tool string) error {
	for _, capability := range requiredCapabilitiesForTool(tool) {
		if !g.IsAllowed(capability) {
			return DisabledError{
				Capability: capability,
				Tool:       tool,
			}
		}
	}
	return nil
}

func (g Gate) RequireExtension(extensionID string) error {
	switch extensionID {
	case "web-search", "email", "mobile-sensors":
		return g.RequireTool(extensionID)
	default:
		return nil
	}
}

func (g Gate) withViperOverride(cfg *viper.Viper, envVar, capability string) Gate {
	if cfg == nil || !cfg.IsSet(envVar) {
		return g
	}

	raw := cfg.GetString(envVar)
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return g
	}
	return g.WithCapability(capability, value)
}

func (g Gate) resolveAllowed() map[string]bool {
	if len(g.allowed) == 0 {
		return NewDefaultGate().allowed
	}
	return g.allowed
}

func requiredCapabilitiesForTool(tool string) []string {
	switch tool {
	case "web-search":
		return []string{CapabilityWebSearch}
	case "email":
		return []string{CapabilityEmail}
	case "mobile-sensors":
		return []string{CapabilityMobileSensors}
	case "screen-capture":
		return []string{CapabilityMobileSensors, CapabilityScreenCapture}
	case "audio-capture":
		return []string{CapabilityMobileSensors, CapabilityAudioCapture}
	default:
		return nil
	}
}
