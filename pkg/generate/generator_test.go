package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProfile(t *testing.T, dir string) string {
	t.Helper()
	profileDir := filepath.Join(dir, "profiles", "test")
	require.NoError(t, os.MkdirAll(profileDir, 0o750))

	content := []byte(`name: test
platform: kubernetes

components:
  controller:
    namespace: my-ns
    deployment: my-controller
    label_selector: app=my-controller
    cluster_role_binding: my-crb
  webhook-only:
    namespace: my-ns
    deployment: my-webhook
    webhook_name: my-webhook-config
    webhook_type: validating
    webhook_resource_kind: ValidatingWebhookConfiguration
`)
	path := filepath.Join(profileDir, "profile.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}

func setupTestTemplates(t *testing.T, dir string) string {
	t.Helper()
	tmplDir := filepath.Join(dir, "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0o750))

	podKill := []byte(`# requires: namespace, deployment, label_selector
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ${COMPONENT}-pod-kill
spec:
  target:
    operator: ${COMPONENT}
    component: ${COMPONENT_NAME}
    resource: Deployment/${DEPLOYMENT}
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: ${DEPLOYMENT}
        namespace: ${NAMESPACE}
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: ${LABEL_SELECTOR}
    ttl: "30s"
  hypothesis:
    description: Pod kill test for ${COMPONENT}
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - ${NAMESPACE}
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "pod-kill.yaml"), podKill, 0o644))

	webhookDisrupt := []byte(`# requires: namespace, deployment, webhook_name, webhook_type, webhook_resource_kind
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ${COMPONENT}-webhook-disrupt
spec:
  target:
    operator: ${COMPONENT}
    component: ${COMPONENT_NAME}
    resource: ${WEBHOOK_RESOURCE_KIND}/${WEBHOOK_NAME}
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: ${WEBHOOK_NAME}
      webhookType: ${WEBHOOK_TYPE}
    ttl: "60s"
  hypothesis:
    description: Webhook disrupt test for ${COMPONENT}
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - ${NAMESPACE}
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "webhook-disrupt.yaml"), webhookDisrupt, 0o644))

	routeTemplate := []byte(`# requires: namespace, deployment, route_name, route_namespace
# platform: openshift
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ${COMPONENT}-route-test
spec:
  target:
    operator: ${COMPONENT}
    component: ${COMPONENT_NAME}
  hypothesis:
    description: Route test
    recoveryTimeout: 120s
  injection:
    type: CRDMutation
    ttl: "60s"
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - ${NAMESPACE}
      - ${ROUTE_NAMESPACE}
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "route-test.yaml"), routeTemplate, 0o644))

	return tmplDir
}

func TestGenerate_BasicOutput(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := Generate(opts)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outDir, "controller", "pod-kill.yaml"))
	_, err = os.Stat(filepath.Join(outDir, "controller", "webhook-disrupt.yaml"))
	assert.True(t, os.IsNotExist(err))

	assert.FileExists(t, filepath.Join(outDir, "webhook-only", "webhook-disrupt.yaml"))
	_, err = os.Stat(filepath.Join(outDir, "webhook-only", "pod-kill.yaml"))
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(outDir, "controller", "route-test.yaml"))
	assert.True(t, os.IsNotExist(err))

	data, err := os.ReadFile(filepath.Join(outDir, "controller", "pod-kill.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "name: controller-pod-kill")
	assert.Contains(t, content, "namespace: my-ns")
	assert.Contains(t, content, "name: my-controller")
	assert.NotContains(t, content, "${")
	assert.NotContains(t, content, "# requires:", "template header comments should be stripped from output")

	// 2 components x 3 templates:
	// pod-kill: controller matches (has label_selector), webhook-only skipped (no label_selector) = 1 gen, 1 skip
	// webhook-disrupt: controller skipped (no webhook_name), webhook-only matches = 1 gen, 1 skip
	// route-test: platform mismatch (openshift vs kubernetes) = 0 gen, 2 skip
	assert.Equal(t, 2, result.Generated)
	assert.Equal(t, 4, result.Skipped)
	assert.Equal(t, 2, result.Components)
	assert.Empty(t, result.Warnings)
}

func TestGenerate_SingleComponent(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
		Component:   "controller",
	}

	_, err := Generate(opts)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outDir, "controller", "pod-kill.yaml"))
	_, err = os.Stat(filepath.Join(outDir, "webhook-only"))
	assert.True(t, os.IsNotExist(err))
}

func TestGenerate_SingleTemplate(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath:  profilePath,
		TemplateDir:  tmplDir,
		OutputDir:    outDir,
		TemplateName: "pod-kill.yaml",
	}

	_, err := Generate(opts)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outDir, "controller", "pod-kill.yaml"))
	_, err = os.Stat(filepath.Join(outDir, "webhook-only", "webhook-disrupt.yaml"))
	assert.True(t, os.IsNotExist(err))
}

func TestGenerate_DryRun(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		DryRun:      true,
	}

	result, err := Generate(opts)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Generated)
	require.Equal(t, 2, len(result.Plan))
	assert.Equal(t, "controller", result.Plan[0].Component)
	assert.Equal(t, "pod-kill.yaml", result.Plan[0].Template)
	assert.Equal(t, "template", result.Plan[0].Source)
	assert.Equal(t, "webhook-only", result.Plan[1].Component)
	assert.Equal(t, "webhook-disrupt.yaml", result.Plan[1].Template)
	assert.Equal(t, "template", result.Plan[1].Source)
}

func TestGenerate_SetVarOverride(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
		SetVars:     []string{"controller:namespace=overridden-ns"},
	}

	_, err := Generate(opts)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outDir, "controller", "pod-kill.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "namespace: overridden-ns")
	assert.NotContains(t, content, "namespace: my-ns")
}

func TestGenerate_ProfileSpecificExperiments(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	profileExpDir := filepath.Join(filepath.Dir(profilePath), "experiments", "controller")
	require.NoError(t, os.MkdirAll(profileExpDir, 0o750))
	custom := []byte(`apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: controller-custom
spec:
  target:
    operator: controller
    component: controller
  injection:
    type: PodKill
    ttl: "30s"
  hypothesis:
    description: Custom experiment
    recoveryTimeout: 60s
  blastRadius:
    maxPodsAffected: 1
`)
	require.NoError(t, os.WriteFile(filepath.Join(profileExpDir, "custom.yaml"), custom, 0o644))

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := Generate(opts)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outDir, "controller", "custom.yaml"))
	assert.FileExists(t, filepath.Join(outDir, "controller", "pod-kill.yaml"))

	copied, err := os.ReadFile(filepath.Join(outDir, "controller", "custom.yaml"))
	require.NoError(t, err)
	assert.Equal(t, string(custom), string(copied), "profile-specific experiment should be byte-for-byte copied")
	assert.Equal(t, 1, result.Copied)
}

func TestGenerate_ProfileSpecificOverridesTemplate(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	profileExpDir := filepath.Join(filepath.Dir(profilePath), "experiments", "controller")
	require.NoError(t, os.MkdirAll(profileExpDir, 0o750))
	override := []byte(`apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: controller-pod-kill-custom
`)
	require.NoError(t, os.WriteFile(filepath.Join(profileExpDir, "pod-kill.yaml"), override, 0o644))

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	_, err := Generate(opts)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outDir, "controller", "pod-kill.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "controller-pod-kill-custom")
	assert.NotContains(t, content, "labelSelector:", "template-generated version should not be present")
}

func TestGenerate_NonexistentComponent(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
		Component:   "nonexistent",
	}

	_, err := Generate(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in profile")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestGenerate_TemplatePathTraversal(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath:  profilePath,
		TemplateDir:  tmplDir,
		OutputDir:    outDir,
		TemplateName: "../profiles/test/profile.yaml",
	}

	_, err := Generate(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestGenerate_DryRunNoFilesWritten(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
		DryRun:      true,
	}

	result, err := Generate(opts)
	require.NoError(t, err)
	assert.Greater(t, result.Generated, 0)

	_, err = os.Stat(outDir)
	assert.True(t, os.IsNotExist(err), "dry-run should not create output directory")
}

func TestGenerate_DeterministicPlanOrder(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		DryRun:      true,
	}

	result1, err := Generate(opts)
	require.NoError(t, err)

	result2, err := Generate(opts)
	require.NoError(t, err)

	require.Equal(t, len(result1.Plan), len(result2.Plan))
	for i := range result1.Plan {
		assert.Equal(t, result1.Plan[i].Component, result2.Plan[i].Component)
		assert.Equal(t, result1.Plan[i].Template, result2.Plan[i].Template)
	}
}

func TestParseSetVar(t *testing.T) {
	comp, field, val, err := parseSetVar("dashboard:namespace=my-ns")
	require.NoError(t, err)
	assert.Equal(t, "dashboard", comp)
	assert.Equal(t, "namespace", field)
	assert.Equal(t, "my-ns", val)
}

func TestParseSetVar_ValueWithEquals(t *testing.T) {
	comp, field, val, err := parseSetVar("ctrl:label_selector=app=test")
	require.NoError(t, err)
	assert.Equal(t, "ctrl", comp)
	assert.Equal(t, "label_selector", field)
	assert.Equal(t, "app=test", val)
}

func TestParseSetVar_InvalidFormat(t *testing.T) {
	_, _, _, err := parseSetVar("invalid-format")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected component:field=value")
}

func TestParseSetVar_MissingColon(t *testing.T) {
	_, _, _, err := parseSetVar("nocomp=value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing ':'")
}

func TestParseSetVar_EmptyValue(t *testing.T) {
	_, _, _, err := parseSetVar("ctrl:namespace=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value must be non-empty")
}

func TestParseSetVar_EmptyComponent(t *testing.T) {
	_, _, _, err := parseSetVar(":namespace=val")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component and field must be non-empty")
}

func TestParseSetVar_EmptyField(t *testing.T) {
	_, _, _, err := parseSetVar("ctrl:=val")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component and field must be non-empty")
}

func TestApplySetVars_UnknownComponent(t *testing.T) {
	p := &Profile{
		Name: "test",
		Components: map[string]ComponentConfig{
			"ctrl": {Namespace: "ns"},
		},
	}

	err := applySetVars(p, []string{"unknown:namespace=val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in profile")
}

func TestApplySetVars_UnknownField(t *testing.T) {
	p := &Profile{
		Name: "test",
		Components: map[string]ComponentConfig{
			"ctrl": {Namespace: "ns"},
		},
	}

	err := applySetVars(p, []string{"ctrl:bogus_field=val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestParseSetVar_NewlineRejected(t *testing.T) {
	_, _, _, err := parseSetVar("ctrl:namespace=evil\ninjected")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newline")
}

func TestApplySetVars_DuplicateFieldLastWins(t *testing.T) {
	p := &Profile{
		Name: "test",
		Components: map[string]ComponentConfig{
			"ctrl": {Namespace: "original"},
		},
	}

	err := applySetVars(p, []string{"ctrl:namespace=first", "ctrl:namespace=second"})
	require.NoError(t, err)
	assert.Equal(t, "second", p.Components["ctrl"].Namespace)
}

func TestSetComponentField_AllFields(t *testing.T) {
	tests := []struct {
		field    string
		getValue func(ComponentConfig) string
	}{
		{"namespace", func(c ComponentConfig) string { return c.Namespace }},
		{"deployment", func(c ComponentConfig) string { return c.Deployment }},
		{"component_name", func(c ComponentConfig) string { return c.ComponentName }},
		{"label_selector", func(c ComponentConfig) string { return c.LabelSelector }},
		{"cluster_role_binding", func(c ComponentConfig) string { return c.ClusterRoleBinding }},
		{"webhook_name", func(c ComponentConfig) string { return c.WebhookName }},
		{"webhook_type", func(c ComponentConfig) string { return c.WebhookType }},
		{"webhook_resource_kind", func(c ComponentConfig) string { return c.WebhookResourceKind }},
		{"webhook_cert_secret", func(c ComponentConfig) string { return c.WebhookCertSecret }},
		{"lease_name", func(c ComponentConfig) string { return c.LeaseName }},
		{"route_name", func(c ComponentConfig) string { return c.RouteName }},
		{"route_namespace", func(c ComponentConfig) string { return c.RouteNamespace }},
		{"config_map_name", func(c ComponentConfig) string { return c.ConfigMapName }},
		{"config_map_key", func(c ComponentConfig) string { return c.ConfigMapKey }},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			cc := ComponentConfig{}
			err := setComponentField(&cc, tt.field, "test-value")
			require.NoError(t, err)
			assert.Equal(t, "test-value", tt.getValue(cc))
		})
	}
}

func TestGenerate_OrphanProfileExperimentWarning(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	orphanDir := filepath.Join(filepath.Dir(profilePath), "experiments", "nonexistent-component")
	require.NoError(t, os.MkdirAll(orphanDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "test.yaml"), []byte("apiVersion: v1\n"), 0o644))

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := Generate(opts)
	require.NoError(t, err)
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "nonexistent-component")
	assert.Contains(t, result.Warnings[0], "no matching component")

	assert.FileExists(t, filepath.Join(outDir, "nonexistent-component", "test.yaml"),
		"orphan experiments should still be copied")
	assert.Equal(t, 1, result.Copied)
}

func TestGenerate_SingleComponentCount(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	tmplDir := setupTestTemplates(t, dir)
	outDir := filepath.Join(dir, "output")

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
		Component:   "controller",
	}

	result, err := Generate(opts)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Components)
}

func TestGenerate_EmptyTemplateDir(t *testing.T) {
	dir := t.TempDir()
	profilePath := setupTestProfile(t, dir)
	emptyTmplDir := filepath.Join(dir, "empty-templates")
	require.NoError(t, os.MkdirAll(emptyTmplDir, 0o750))

	opts := GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: emptyTmplDir,
		OutputDir:   filepath.Join(dir, "output"),
	}

	_, err := Generate(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template files found")
}
