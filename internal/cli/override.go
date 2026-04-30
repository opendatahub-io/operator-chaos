package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
)

// applyOverrides applies a slice of "key=value" overrides to the experiment's
// Spec using dot-path notation. The experiment is converted to an unstructured
// map, the overrides are applied by walking the dot-path, and then the map is
// marshalled back into the Spec struct.
//
// Supported paths use dots for nested fields and [N] for array indices:
//
//	injection.parameters.name=foo
//	blastRadius.allowedNamespaces[0]=my-ns
//	target.resource=Route/new-name
func applyOverrides(exp *v1alpha1.ChaosExperiment, overrides []string) error {
	if len(overrides) == 0 {
		return nil
	}

	// Parse all overrides first so we fail fast on bad input.
	parsed := make([]struct {
		key   string
		value string
	}, len(overrides))
	for i, o := range overrides {
		idx := strings.IndexByte(o, '=')
		if idx < 1 {
			return fmt.Errorf("invalid override %q: expected key=value format", o)
		}
		parsed[i].key = o[:idx]
		parsed[i].value = o[idx+1:]
	}

	// Marshal the Spec to JSON, then unmarshal into a generic map.
	specJSON, err := json.Marshal(exp.Spec)
	if err != nil {
		return fmt.Errorf("marshalling spec: %w", err)
	}
	var specMap map[string]interface{}
	if err := json.Unmarshal(specJSON, &specMap); err != nil {
		return fmt.Errorf("unmarshalling spec to map: %w", err)
	}

	// Apply each override.
	for _, p := range parsed {
		segments := parseDotPath(p.key)
		if err := setNestedValue(specMap, segments, p.value); err != nil {
			return fmt.Errorf("setting %q: %w", p.key, err)
		}
	}

	// Marshal the map back to JSON and unmarshal into the Spec struct.
	modifiedJSON, err := json.Marshal(specMap)
	if err != nil {
		return fmt.Errorf("marshalling modified spec: %w", err)
	}
	if err := json.Unmarshal(modifiedJSON, &exp.Spec); err != nil {
		return fmt.Errorf("unmarshalling modified spec: %w", err)
	}

	return nil
}

// pathSegment represents a single segment of a dot-path. When index >= 0 the
// segment refers to an array element (e.g. "allowedNamespaces[0]").
type pathSegment struct {
	key   string
	index int // -1 means "not an array access"
}

// parseDotPath splits a dot-separated path into segments, handling array
// bracket notation like "blastRadius.allowedNamespaces[0]".
func parseDotPath(path string) []pathSegment {
	parts := strings.Split(path, ".")
	segments := make([]pathSegment, 0, len(parts))
	for _, p := range parts {
		seg := pathSegment{key: p, index: -1}
		if bracketIdx := strings.IndexByte(p, '['); bracketIdx >= 0 {
			seg.key = p[:bracketIdx]
			inner := strings.TrimSuffix(p[bracketIdx+1:], "]")
			if n, err := strconv.Atoi(inner); err == nil {
				seg.index = n
			}
		}
		segments = append(segments, seg)
	}
	return segments
}

// setNestedValue walks the map along the given path segments and sets the leaf
// to the provided value string.
func setNestedValue(m map[string]interface{}, segments []pathSegment, value string) error {
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Walk to the parent of the target field.
	current := interface{}(m)
	for i, seg := range segments[:len(segments)-1] {
		current = resolveSegment(current, seg)
		if current == nil {
			return fmt.Errorf("path segment %q (position %d) not found", seg.key, i)
		}
	}

	// Set the leaf value.
	last := segments[len(segments)-1]
	switch parent := current.(type) {
	case map[string]interface{}:
		if last.index >= 0 {
			arr, ok := parent[last.key].([]interface{})
			if !ok {
				return fmt.Errorf("field %q is not an array", last.key)
			}
			if last.index >= len(arr) {
				return fmt.Errorf("index %d out of range for %q (length %d)", last.index, last.key, len(arr))
			}
			arr[last.index] = value
		} else {
			parent[last.key] = value
		}
	default:
		return fmt.Errorf("cannot set value on non-object parent (type %T)", parent)
	}

	return nil
}

// resolveSegment returns the child value for a given segment. When the segment
// has an array index it first looks up the key in the map, then indexes into
// the resulting slice.
func resolveSegment(current interface{}, seg pathSegment) interface{} {
	m, ok := current.(map[string]interface{})
	if !ok {
		return nil
	}
	child, exists := m[seg.key]
	if !exists {
		return nil
	}
	if seg.index >= 0 {
		arr, ok := child.([]interface{})
		if !ok || seg.index >= len(arr) {
			return nil
		}
		return arr[seg.index]
	}
	return child
}
