package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TemplateHeader holds parsed metadata from template comment lines.
type TemplateHeader struct {
	Requires []string
	Platform string
}

// Template represents a parsed experiment template file.
type Template struct {
	Name    string
	Path    string
	Header  TemplateHeader
	Content string
}

var varPattern = regexp.MustCompile(`\$\{([A-Z_]+)\}`)

// anyVarPattern matches any ${...} including lowercase, used to detect typos.
var anyVarPattern = regexp.MustCompile(`\$\{[^}]+\}`)

// parseTemplateHeader extracts # requires: and # platform: from the first 5 lines.
func parseTemplateHeader(content string) TemplateHeader {
	var h TemplateHeader
	lines := strings.SplitN(content, "\n", 6)
	if len(lines) > 5 {
		lines = lines[:5]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# requires:") {
			fields := strings.TrimPrefix(trimmed, "# requires:")
			for _, f := range strings.Split(fields, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					h.Requires = append(h.Requires, f)
				}
			}
		} else if strings.HasPrefix(trimmed, "# platform:") {
			h.Platform = strings.TrimSpace(strings.TrimPrefix(trimmed, "# platform:"))
		}
	}

	return h
}

// fieldToVar converts a snake_case profile field name to an UPPER_CASE template variable.
func fieldToVar(field string) string {
	return strings.ToUpper(field)
}

// matchesComponent checks whether a component's variables satisfy a template's requirements.
// A variable must be present AND non-empty to match.
func matchesComponent(requires []string, vars map[string]string) bool {
	for _, req := range requires {
		varName := fieldToVar(req)
		val, ok := vars[varName]
		if !ok || val == "" {
			return false
		}
	}
	return true
}

// matchesPlatform checks whether a profile's platform satisfies a template's platform requirement.
func matchesPlatform(templatePlatform, profilePlatform string) bool {
	if templatePlatform == "" {
		return true
	}
	return templatePlatform == profilePlatform
}

// stripHeaderComments removes # requires: and # platform: lines from the first 5 lines
// of template content, since they are template metadata not relevant to generated output.
func stripHeaderComments(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i < 5 && (strings.HasPrefix(trimmed, "# requires:") || strings.HasPrefix(trimmed, "# platform:")) {
			continue
		}
		result = append(result, line)
	}
	output := strings.Join(result, "\n")
	return strings.TrimLeft(output, "\n")
}

// substituteVariables replaces all ${VAR} placeholders in content with values from vars.
// Returns an error if any ${...} placeholders remain after substitution or if values
// contain characters that would break YAML structure.
func substituteVariables(content string, vars map[string]string) (string, error) {
	for k, v := range vars {
		if strings.ContainsAny(v, "\n\r") {
			return "", fmt.Errorf("variable %s value contains newline characters (YAML injection risk)", k)
		}
	}

	result := varPattern.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[2 : len(match)-1]
		if val, ok := vars[varName]; ok {
			return val
		}
		return match
	})

	remaining := varPattern.FindAllString(result, -1)
	if len(remaining) > 0 {
		return "", fmt.Errorf("unresolved variables: %s", strings.Join(remaining, ", "))
	}

	// Detect ${lowercase} or other non-standard patterns that survived substitution.
	nonStandard := anyVarPattern.FindAllString(result, -1)
	if len(nonStandard) > 0 {
		return "", fmt.Errorf("unresolved variable placeholders (use UPPER_CASE): %s", strings.Join(nonStandard, ", "))
	}

	return result, nil
}

// LoadTemplate reads and parses a template YAML file.
func LoadTemplate(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", path, err)
	}

	content := string(data)
	header := parseTemplateHeader(content)

	for _, req := range header.Requires {
		if !IsRecognizedField(req) {
			return nil, fmt.Errorf("template %s: unrecognized field %q in # requires: directive", path, req)
		}
	}

	name := filepath.Base(path)

	return &Template{
		Name:    name,
		Path:    path,
		Header:  header,
		Content: content,
	}, nil
}

// LoadTemplates reads all .yaml files from a directory.
func LoadTemplates(dir string) ([]*Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading templates directory %s: %w", dir, err)
	}

	var templates []*Template
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		tmpl, err := LoadTemplate(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}
