package policy

import "testing"

func TestDefaultGateDisablesSensitiveCapabilities(t *testing.T) {
	t.Parallel()

	gate := NewDefaultGate()
	if !gate.IsAllowed(CapabilityWebSearch) {
		t.Fatalf("expected web-search capability enabled by default")
	}
	if gate.IsAllowed(CapabilityEmail) {
		t.Fatalf("expected email capability disabled by default")
	}
	if gate.IsAllowed(CapabilityMobileSensors) {
		t.Fatalf("expected mobile-sensors capability disabled by default")
	}
}

func TestLoadFromEnvOverridesDefaults(t *testing.T) {
	t.Setenv("AGENT_CAPABILITY_EMAIL", "true")
	t.Setenv("AGENT_CAPABILITY_SCREEN_CAPTURE", "true")
	t.Setenv("AGENT_CAPABILITY_MOBILE_SENSORS", "true")

	gate := LoadFromEnv()
	if !gate.IsAllowed(CapabilityEmail) {
		t.Fatalf("expected email capability enabled from env")
	}
	if !gate.IsAllowed(CapabilityMobileSensors) {
		t.Fatalf("expected mobile-sensors capability enabled from env")
	}
	if !gate.IsAllowed(CapabilityScreenCapture) {
		t.Fatalf("expected screen-capture capability enabled from env")
	}
}

func TestRequireToolChecksCapabilityChain(t *testing.T) {
	t.Parallel()

	gate := NewDefaultGate().
		WithCapability(CapabilityMobileSensors, true).
		WithCapability(CapabilityScreenCapture, false)
	err := gate.RequireTool("screen-capture")
	if err == nil {
		t.Fatalf("expected screen-capture to be denied")
	}

	deniedErr, ok := err.(DisabledError)
	if !ok {
		t.Fatalf("expected DisabledError, got %T", err)
	}
	if deniedErr.Capability != CapabilityScreenCapture {
		t.Fatalf("expected denied capability %q, got %q", CapabilityScreenCapture, deniedErr.Capability)
	}
}
