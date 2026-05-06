package generate

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Profile defines variable values for generating experiments for a specific operator distribution.
type Profile struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Version     string                     `json:"version,omitempty"`
	Platform    string                     `json:"platform,omitempty"`
	Components  map[string]ComponentConfig `json:"components"`
	Warnings    []string                   `json:"-"`
}

// ComponentConfig defines the operator-specific values for a single component.
type ComponentConfig struct {
	Namespace          string `json:"namespace,omitempty"`
	Deployment         string `json:"deployment,omitempty"`
	ComponentName      string `json:"component_name,omitempty"`
	LabelSelector      string `json:"label_selector,omitempty"`
	ClusterRoleBinding string `json:"cluster_role_binding,omitempty"`
	WebhookName        string `json:"webhook_name,omitempty"`
	WebhookType        string `json:"webhook_type,omitempty"`
	WebhookResourceKind string `json:"webhook_resource_kind,omitempty"`
	WebhookCertSecret  string `json:"webhook_cert_secret,omitempty"`
	LeaseName          string `json:"lease_name,omitempty"`
	RouteName          string `json:"route_name,omitempty"`
	RouteNamespace     string `json:"route_namespace,omitempty"`
	ConfigMapName      string `json:"config_map_name,omitempty"`
	ConfigMapKey       string `json:"config_map_key,omitempty"`
}

// Variables returns a map of template variable names to values for this component.
// The componentKey is the map key name from the profile.
// The profileName is the top-level profile name used as the operator identifier.
// Only non-empty values are included.
func (c ComponentConfig) Variables(componentKey, profileName string) map[string]string {
	vars := map[string]string{
		"COMPONENT": componentKey,
		"OPERATOR":  profileName,
	}

	compName := c.ComponentName
	if compName == "" {
		compName = componentKey
	}
	vars["COMPONENT_NAME"] = compName

	fields := map[string]string{
		"NAMESPACE":            c.Namespace,
		"DEPLOYMENT":           c.Deployment,
		"LABEL_SELECTOR":       c.LabelSelector,
		"CLUSTER_ROLE_BINDING": c.ClusterRoleBinding,
		"WEBHOOK_NAME":         c.WebhookName,
		"WEBHOOK_TYPE":         c.WebhookType,
		"WEBHOOK_RESOURCE_KIND": c.WebhookResourceKind,
		"WEBHOOK_CERT_SECRET":  c.WebhookCertSecret,
		"LEASE_NAME":           c.LeaseName,
		"ROUTE_NAME":           c.RouteName,
		"ROUTE_NAMESPACE":      c.RouteNamespace,
		"CONFIG_MAP_NAME":      c.ConfigMapName,
		"CONFIG_MAP_KEY":       c.ConfigMapKey,
	}

	for k, v := range fields {
		if v != "" {
			vars[k] = v
		}
	}

	return vars
}

// knownProfileFields lists fields recognized at the top level of a profile.
var knownProfileFields = map[string]bool{
	"name": true, "description": true, "version": true, "platform": true, "components": true,
}

// knownComponentFields lists fields recognized in a component definition.
// This is the single source of truth; recognizedFields and IsRecognizedField use it.
var knownComponentFields = map[string]bool{
	"namespace": true, "deployment": true, "component_name": true,
	"label_selector": true, "cluster_role_binding": true,
	"webhook_name": true, "webhook_type": true, "webhook_resource_kind": true,
	"webhook_cert_secret": true, "lease_name": true,
	"route_name": true, "route_namespace": true,
	"config_map_name": true, "config_map_key": true,
}

// LoadProfile reads and validates a profile YAML file.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile %s: %w", path, err)
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile %s: %w", path, err)
	}

	if err := validateProfile(&p); err != nil {
		return nil, fmt.Errorf("validating profile %s: %w", path, err)
	}

	p.Warnings = detectUnknownFields(data)

	// Default component_name to key name
	for key, comp := range p.Components {
		if comp.ComponentName == "" {
			comp.ComponentName = key
			p.Components[key] = comp
		}
	}

	return &p, nil
}

func detectUnknownFields(data []byte) []string {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}

	var warnings []string
	for k := range raw {
		if !knownProfileFields[k] {
			warnings = append(warnings, fmt.Sprintf("unknown profile field %q (will be ignored)", k))
		}
	}

	comps, ok := raw["components"].(map[string]interface{})
	if !ok {
		return warnings
	}

	for compKey, compVal := range comps {
		compMap, ok := compVal.(map[string]interface{})
		if !ok {
			continue
		}
		for fieldKey := range compMap {
			if !knownComponentFields[fieldKey] {
				warnings = append(warnings, fmt.Sprintf("component %q: unknown field %q (will be ignored)", compKey, fieldKey))
			}
		}
	}

	sort.Strings(warnings)
	return warnings
}

func validateProfile(p *Profile) error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(p.Components) == 0 {
		return fmt.Errorf("at least one component is required")
	}

	for key := range p.Components {
		if key == "" {
			return fmt.Errorf("empty component name")
		}
		if strings.Contains(key, "..") || strings.Contains(key, "/") || strings.Contains(key, "\\") {
			return fmt.Errorf("component name %q contains path traversal characters", key)
		}
	}

	return nil
}

// IsRecognizedField returns whether a field name is in the fixed variable mapping.
func IsRecognizedField(name string) bool {
	return knownComponentFields[name]
}
