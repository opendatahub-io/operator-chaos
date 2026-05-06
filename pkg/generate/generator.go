package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type GenerateOptions struct {
	ProfilePath  string
	TemplateDir  string
	OutputDir    string
	Component    string
	TemplateName string
	SetVars      []string
	DryRun       bool
}

type GenerateResult struct {
	Generated  int
	Skipped    int
	Copied     int
	Components int
	Warnings   []string
	Plan       []PlannedFile
}

type PlannedFile struct {
	Component string
	Template  string
	Source    string
}

func Generate(opts GenerateOptions) (*GenerateResult, error) {
	profile, err := LoadProfile(opts.ProfilePath)
	if err != nil {
		return nil, err
	}

	if err := applySetVars(profile, opts.SetVars); err != nil {
		return nil, err
	}

	var templates []*Template
	if opts.TemplateName != "" {
		if filepath.Base(opts.TemplateName) != opts.TemplateName || strings.ContainsAny(opts.TemplateName, "/\\") || strings.Contains(opts.TemplateName, "..") {
			return nil, fmt.Errorf("template name %q must not contain path separators or traversal characters", opts.TemplateName)
		}
		tmpl, err := LoadTemplate(filepath.Join(opts.TemplateDir, opts.TemplateName))
		if err != nil {
			return nil, err
		}
		templates = []*Template{tmpl}
	} else {
		templates, err = LoadTemplates(opts.TemplateDir)
		if err != nil {
			return nil, err
		}
		if len(templates) == 0 {
			return nil, fmt.Errorf("no template files found in %s", opts.TemplateDir)
		}
	}

	if opts.Component != "" {
		if _, ok := profile.Components[opts.Component]; !ok {
			available := make([]string, 0, len(profile.Components))
			for k := range profile.Components {
				available = append(available, k)
			}
			sort.Strings(available)
			return nil, fmt.Errorf("component %q not found in profile (available: %v)", opts.Component, available)
		}
	}

	profileDir := filepath.Dir(opts.ProfilePath)
	profileExpDir := filepath.Join(profileDir, "experiments")
	profileExps := scanProfileExperiments(profileExpDir)

	compKeys := make([]string, 0, len(profile.Components))
	for k := range profile.Components {
		compKeys = append(compKeys, k)
	}
	sort.Strings(compKeys)

	result := &GenerateResult{
		Components: len(compKeys),
		Warnings:   append([]string{}, profile.Warnings...),
	}
	if opts.Component != "" {
		result.Components = 1
	}

	for _, tmpl := range templates {
		if !matchesPlatform(tmpl.Header.Platform, profile.Platform) {
			for _, compKey := range compKeys {
				if opts.Component != "" && compKey != opts.Component {
					continue
				}
				result.Skipped++
			}
			continue
		}

		for _, compKey := range compKeys {
			comp := profile.Components[compKey]
			if opts.Component != "" && compKey != opts.Component {
				continue
			}

			vars := comp.Variables(compKey, profile.Name)

			if !matchesComponent(tmpl.Header.Requires, vars) {
				result.Skipped++
				continue
			}

			if isProfileSpecific(profileExps, compKey, tmpl.Name) {
				result.Skipped++
				continue
			}

			if opts.DryRun {
				result.Plan = append(result.Plan, PlannedFile{
					Component: compKey,
					Template:  tmpl.Name,
					Source:    "template",
				})
				result.Generated++
				continue
			}

			output, err := substituteVariables(stripHeaderComments(tmpl.Content), vars)
			if err != nil {
				return nil, fmt.Errorf("substituting %s for %s: %w", tmpl.Name, compKey, err)
			}

			outPath := filepath.Join(opts.OutputDir, compKey, tmpl.Name)
			if err := writeFile(outPath, output); err != nil {
				return nil, err
			}
			result.Generated++
		}
	}

	profileExpKeys := make([]string, 0, len(profileExps))
	for k := range profileExps {
		profileExpKeys = append(profileExpKeys, k)
	}
	sort.Strings(profileExpKeys)

	for _, compKey := range profileExpKeys {
		files := profileExps[compKey]
		if opts.Component != "" && compKey != opts.Component {
			continue
		}

		if _, ok := profile.Components[compKey]; !ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("profile experiment directory %q has no matching component in profile", compKey))
		}

		for _, f := range files {
			if opts.DryRun {
				result.Plan = append(result.Plan, PlannedFile{
					Component: compKey,
					Template:  filepath.Base(f),
					Source:    "profile-specific",
				})
				result.Copied++
				continue
			}

			data, err := os.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("reading profile experiment %s: %w", f, err)
			}

			outPath := filepath.Join(opts.OutputDir, compKey, filepath.Base(f))
			if err := writeFile(outPath, string(data)); err != nil {
				return nil, err
			}
			result.Copied++
		}
	}

	return result, nil
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return nil
}

func scanProfileExperiments(profileExpDir string) map[string][]string {
	result := make(map[string][]string)

	entries, err := os.ReadDir(profileExpDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			continue
		}
		compDir := filepath.Join(profileExpDir, name)
		files, err := os.ReadDir(compDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				result[entry.Name()] = append(result[entry.Name()], filepath.Join(compDir, name))
			}
		}
	}

	return result
}

func isProfileSpecific(profileExps map[string][]string, component, templateName string) bool {
	files, ok := profileExps[component]
	if !ok {
		return false
	}
	for _, f := range files {
		if filepath.Base(f) == templateName {
			return true
		}
	}
	return false
}

func parseSetVar(s string) (component, field, value string, err error) {
	eqIdx := strings.Index(s, "=")
	if eqIdx < 0 {
		return "", "", "", fmt.Errorf("invalid --set-var %q: expected component:field=value format", s)
	}

	key := s[:eqIdx]
	value = s[eqIdx+1:]

	colonIdx := strings.Index(key, ":")
	if colonIdx < 0 {
		return "", "", "", fmt.Errorf("invalid --set-var %q: expected component:field=value format (missing ':')", s)
	}

	component = key[:colonIdx]
	field = key[colonIdx+1:]

	if component == "" || field == "" {
		return "", "", "", fmt.Errorf("invalid --set-var %q: component and field must be non-empty", s)
	}

	if value == "" {
		return "", "", "", fmt.Errorf("invalid --set-var %q: value must be non-empty", s)
	}

	if strings.ContainsAny(value, "\n\r") {
		return "", "", "", fmt.Errorf("invalid --set-var %q: value must not contain newline characters", s)
	}

	return component, field, value, nil
}

func applySetVars(profile *Profile, setVars []string) error {
	for _, sv := range setVars {
		comp, field, val, err := parseSetVar(sv)
		if err != nil {
			return err
		}

		cc, ok := profile.Components[comp]
		if !ok {
			return fmt.Errorf("--set-var: component %q not found in profile", comp)
		}

		if err := setComponentField(&cc, field, val); err != nil {
			return fmt.Errorf("--set-var %s: %w", sv, err)
		}
		profile.Components[comp] = cc
	}
	return nil
}

func setComponentField(cc *ComponentConfig, field, value string) error {
	switch field {
	case "namespace":
		cc.Namespace = value
	case "deployment":
		cc.Deployment = value
	case "component_name":
		cc.ComponentName = value
	case "label_selector":
		cc.LabelSelector = value
	case "cluster_role_binding":
		cc.ClusterRoleBinding = value
	case "webhook_name":
		cc.WebhookName = value
	case "webhook_type":
		cc.WebhookType = value
	case "webhook_resource_kind":
		cc.WebhookResourceKind = value
	case "webhook_cert_secret":
		cc.WebhookCertSecret = value
	case "lease_name":
		cc.LeaseName = value
	case "route_name":
		cc.RouteName = value
	case "route_namespace":
		cc.RouteNamespace = value
	case "config_map_name":
		cc.ConfigMapName = value
	case "config_map_key":
		cc.ConfigMapKey = value
	default:
		return fmt.Errorf("unknown field %q", field)
	}
	return nil
}
