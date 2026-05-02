package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplateHeader(t *testing.T) {
	content := `# requires: namespace, deployment, label_selector
# platform: openshift
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
`
	h := parseTemplateHeader(content)
	assert.Equal(t, []string{"namespace", "deployment", "label_selector"}, h.Requires)
	assert.Equal(t, "openshift", h.Platform)
}

func TestParseTemplateHeader_NoRequires(t *testing.T) {
	content := `apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
`
	h := parseTemplateHeader(content)
	assert.Empty(t, h.Requires)
	assert.Empty(t, h.Platform)
}

func TestParseTemplateHeader_RequiresOnly(t *testing.T) {
	content := `# requires: namespace, deployment
apiVersion: chaos.operatorchaos.io/v1alpha1
`
	h := parseTemplateHeader(content)
	assert.Equal(t, []string{"namespace", "deployment"}, h.Requires)
	assert.Empty(t, h.Platform)
}

func TestSubstituteVariables(t *testing.T) {
	tmpl := `name: ${COMPONENT}-pod-kill
namespace: ${NAMESPACE}
deployment: ${DEPLOYMENT}
`
	vars := map[string]string{
		"COMPONENT":  "dashboard",
		"NAMESPACE":  "my-ns",
		"DEPLOYMENT": "my-deploy",
	}

	expected := `name: dashboard-pod-kill
namespace: my-ns
deployment: my-deploy
`
	result, err := substituteVariables(tmpl, vars)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestSubstituteVariables_UnresolvedError(t *testing.T) {
	tmpl := `name: ${COMPONENT}-test
namespace: ${UNKNOWN_VAR}
`
	vars := map[string]string{"COMPONENT": "test"}

	_, err := substituteVariables(tmpl, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNKNOWN_VAR")
}

func TestParseTemplateHeader_RequiresOnLine6Ignored(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n# requires: namespace, deployment\n"
	h := parseTemplateHeader(content)
	assert.Empty(t, h.Requires, "requires on line 6+ should be ignored")
}

func TestParseTemplateHeader_RequiresOnLine5(t *testing.T) {
	content := "line1\nline2\nline3\nline4\n# requires: namespace, deployment\n"
	h := parseTemplateHeader(content)
	assert.Equal(t, []string{"namespace", "deployment"}, h.Requires)
}

func TestSubstituteVariables_LowercaseVarError(t *testing.T) {
	tmpl := "name: ${component}-test\n"
	vars := map[string]string{}

	_, err := substituteVariables(tmpl, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UPPER_CASE")
}

func TestSubstituteVariables_NewlineInValueRejected(t *testing.T) {
	tmpl := "namespace: ${NAMESPACE}\n"
	vars := map[string]string{
		"NAMESPACE": "evil-ns\n  extra: injected",
	}

	_, err := substituteVariables(tmpl, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newline")
}

func TestSubstituteVariables_CarriageReturnRejected(t *testing.T) {
	tmpl := "namespace: ${NAMESPACE}\n"
	vars := map[string]string{
		"NAMESPACE": "evil-ns\r  extra: injected",
	}

	_, err := substituteVariables(tmpl, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newline")
}

func TestSubstituteVariables_EmptyContent(t *testing.T) {
	result, err := substituteVariables("", map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestMatchesPlatform(t *testing.T) {
	assert.True(t, matchesPlatform("", ""))
	assert.True(t, matchesPlatform("", "openshift"))
	assert.False(t, matchesPlatform("openshift", ""))
	assert.True(t, matchesPlatform("openshift", "openshift"))
	assert.False(t, matchesPlatform("openshift", "kubernetes"))
}

func TestMatchesComponent(t *testing.T) {
	tests := []struct {
		name     string
		requires []string
		vars     map[string]string
		matches  bool
	}{
		{
			name:     "all required present",
			requires: []string{"namespace", "deployment"},
			vars:     map[string]string{"NAMESPACE": "ns", "DEPLOYMENT": "dep", "COMPONENT": "c", "COMPONENT_NAME": "c"},
			matches:  true,
		},
		{
			name:     "missing required",
			requires: []string{"namespace", "deployment", "webhook_name"},
			vars:     map[string]string{"NAMESPACE": "ns", "DEPLOYMENT": "dep", "COMPONENT": "c", "COMPONENT_NAME": "c"},
			matches:  false,
		},
		{
			name:     "empty value treated as missing",
			requires: []string{"namespace", "deployment"},
			vars:     map[string]string{"NAMESPACE": "ns", "DEPLOYMENT": "", "COMPONENT": "c", "COMPONENT_NAME": "c"},
			matches:  false,
		},
		{
			name:     "no requires matches all",
			requires: nil,
			vars:     map[string]string{"COMPONENT": "c", "COMPONENT_NAME": "c"},
			matches:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesComponent(tt.requires, tt.vars)
			assert.Equal(t, tt.matches, result)
		})
	}
}

func TestLoadTemplate(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`# requires: namespace, deployment
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ${COMPONENT}-pod-kill
`)
	path := filepath.Join(dir, "pod-kill.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	tmpl, err := LoadTemplate(path)
	require.NoError(t, err)
	assert.Equal(t, "pod-kill.yaml", tmpl.Name)
	assert.Equal(t, path, tmpl.Path)
	assert.Equal(t, []string{"namespace", "deployment"}, tmpl.Header.Requires)
	assert.Contains(t, tmpl.Content, "${COMPONENT}")
}

func TestLoadTemplates_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`# requires: namespace, deployment
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: ${COMPONENT}-test
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yml"), content, 0o644))

	templates, err := LoadTemplates(dir)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "test.yml", templates[0].Name)
}

func TestParseTemplateHeader_MultipleRequiresLines(t *testing.T) {
	content := "# requires: namespace, deployment\n# requires: webhook_name\napiVersion: v1\n"
	h := parseTemplateHeader(content)
	assert.Equal(t, []string{"namespace", "deployment", "webhook_name"}, h.Requires)
}

func TestLoadTemplate_UnrecognizedRequiresField(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# requires: namespace, bogus_field\napiVersion: v1\n")
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized field")
	assert.Contains(t, err.Error(), "bogus_field")
}

func TestStripHeaderComments(t *testing.T) {
	content := "# requires: namespace, deployment\n# platform: openshift\napiVersion: v1\nkind: ChaosExperiment\n"
	result := stripHeaderComments(content)
	assert.NotContains(t, result, "# requires:")
	assert.NotContains(t, result, "# platform:")
	assert.Contains(t, result, "apiVersion: v1")
	assert.Contains(t, result, "kind: ChaosExperiment")
	assert.True(t, strings.HasPrefix(result, "apiVersion:"), "should not have leading blank line after stripping headers")
}

func TestStripHeaderComments_PreservesNonHeaderComments(t *testing.T) {
	content := "# requires: namespace\napiVersion: v1\nkind: Test\n# this is a regular comment\nspec:\n"
	result := stripHeaderComments(content)
	assert.NotContains(t, result, "# requires:")
	assert.Contains(t, result, "# this is a regular comment")
}

func TestLoadTemplates_RealTemplateDir(t *testing.T) {
	tmplDir := "../../templates"
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		t.Skip("templates directory not found")
	}

	templates, err := LoadTemplates(tmplDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(templates), 13, "expected at least 13 templates")

	for _, tmpl := range templates {
		t.Run(tmpl.Name, func(t *testing.T) {
			assert.NotEmpty(t, tmpl.Content)
			assert.Contains(t, tmpl.Content, "${COMPONENT}")
			assert.NotEmpty(t, tmpl.Header.Requires, "template %s should have # requires: header", tmpl.Name)
		})
	}
}
