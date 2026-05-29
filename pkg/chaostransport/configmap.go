package chaostransport

import (
	"encoding/json"
	"fmt"
)

const (
	// ChaosConfigMapName is the default name of the ConfigMap holding chaos configuration.
	ChaosConfigMapName = "operator-chaos-config"
	// ChaosConfigKey is the key within the ConfigMap's data that holds the JSON config.
	ChaosConfigKey = "config"
)

// ParseFaultConfigFromData parses a FaultConfig from a ConfigMap's data map.
// Returns an active FaultConfig with no faults if data is nil, empty, or missing the config key.
// The Active field defaults to true (matching NewFaultConfig behavior) unless
// explicitly set to false in the JSON.
func ParseFaultConfigFromData(data map[string]string) (*FaultConfig, error) {
	if data == nil {
		return NewFaultConfig(nil), nil
	}
	configJSON, ok := data[ChaosConfigKey]
	if !ok || configJSON == "" {
		return NewFaultConfig(nil), nil
	}
	cfg := NewFaultConfig(nil)
	if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
		return nil, fmt.Errorf("parsing chaos config: %w", err)
	}
	return cfg, nil
}
