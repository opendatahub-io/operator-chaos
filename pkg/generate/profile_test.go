package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfile(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profiles", "test")
	require.NoError(t, os.MkdirAll(profileDir, 0o750))

	content := []byte(`name: test
description: Test profile
version: "1.0"
platform: kubernetes

components:
  my-controller:
    namespace: default
    deployment: my-controller
    label_selector: app=my-controller
    cluster_role_binding: my-controller-crb
`)
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "profile.yaml"), content, 0o644))

	p, err := LoadProfile(filepath.Join(profileDir, "profile.yaml"))
	require.NoError(t, err)

	assert.Equal(t, "test", p.Name)
	assert.Equal(t, "Test profile", p.Description)
	assert.Equal(t, "1.0", p.Version)
	assert.Equal(t, "kubernetes", p.Platform)
	assert.Len(t, p.Components, 1)

	comp := p.Components["my-controller"]
	assert.Equal(t, "default", comp.Namespace)
	assert.Equal(t, "my-controller", comp.Deployment)
	assert.Equal(t, "app=my-controller", comp.LabelSelector)
	assert.Equal(t, "my-controller-crb", comp.ClusterRoleBinding)
}

func TestLoadProfile_ComponentNameDefault(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`name: test
components:
  workbenches:
    namespace: default
    deployment: odh-notebook-controller-manager
    component_name: odh-notebook-controller
  dashboard:
    namespace: default
    deployment: dashboard
`)
	path := filepath.Join(dir, "profile.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	p, err := LoadProfile(path)
	require.NoError(t, err)

	// Explicit component_name is preserved
	assert.Equal(t, "odh-notebook-controller", p.Components["workbenches"].ComponentName)
	// Missing component_name defaults to key
	assert.Equal(t, "dashboard", p.Components["dashboard"].ComponentName)
}

func TestLoadProfile_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		errMsg  string
	}{
		{
			name:   "empty component name",
			yaml:   "name: test\ncomponents:\n  \"\":\n    namespace: x\n    deployment: x\n",
			errMsg: "empty component name",
		},
		{
			name:   "path traversal dots",
			yaml:   "name: test\ncomponents:\n  ../evil:\n    namespace: x\n    deployment: x\n",
			errMsg: "path traversal",
		},
		{
			name:   "path traversal slash",
			yaml:   "name: test\ncomponents:\n  foo/bar:\n    namespace: x\n    deployment: x\n",
			errMsg: "path traversal",
		},
		{
			name:   "path traversal backslash",
			yaml:   "name: test\ncomponents:\n  foo\\bar:\n    namespace: x\n    deployment: x\n",
			errMsg: "path traversal",
		},
		{
			name:   "missing name",
			yaml:   "components:\n  foo:\n    namespace: x\n    deployment: x\n",
			errMsg: "name is required",
		},
		{
			name:   "zero components",
			yaml:   "name: test\n",
			errMsg: "at least one component",
		},
		{
			name:   "empty components map",
			yaml:   "name: test\ncomponents: {}\n",
			errMsg: "at least one component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profile.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.yaml), 0o644))

			_, err := LoadProfile(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestComponentVariables(t *testing.T) {
	comp := ComponentConfig{
		Namespace:        "my-ns",
		Deployment:       "my-deploy",
		ComponentName:    "my-component",
		LabelSelector:    "app=test",
		ClusterRoleBinding: "my-crb",
	}

	vars := comp.Variables("my-key", "test-operator")
	assert.Equal(t, "my-key", vars["COMPONENT"])
	assert.Equal(t, "test-operator", vars["OPERATOR"])
	assert.Equal(t, "my-component", vars["COMPONENT_NAME"])
	assert.Equal(t, "my-ns", vars["NAMESPACE"])
	assert.Equal(t, "my-deploy", vars["DEPLOYMENT"])
	assert.Equal(t, "app=test", vars["LABEL_SELECTOR"])
	assert.Equal(t, "my-crb", vars["CLUSTER_ROLE_BINDING"])
	// Unset fields should not appear
	_, ok := vars["WEBHOOK_NAME"]
	assert.False(t, ok)
}

func TestLoadProfile_UnknownFieldsTolerated(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`name: test
components:
  ctrl:
    namespace: default
    deployment: ctrl
    future_field: should-be-ignored
`)
	path := filepath.Join(dir, "profile.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	p, err := LoadProfile(path)
	require.NoError(t, err)
	assert.Equal(t, "test", p.Name)
	assert.Equal(t, "default", p.Components["ctrl"].Namespace)

	vars := p.Components["ctrl"].Variables("ctrl", "test")
	_, hasFutureField := vars["FUTURE_FIELD"]
	assert.False(t, hasFutureField, "unknown fields should not leak into Variables()")

	require.Len(t, p.Warnings, 1)
	assert.Contains(t, p.Warnings[0], "future_field")
	assert.Contains(t, p.Warnings[0], "will be ignored")
}

func TestLoadProfile_FileNotFound(t *testing.T) {
	_, err := LoadProfile("/nonexistent/profile.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading profile")
}

func TestLoadProfile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	require.NoError(t, os.WriteFile(path, []byte("[\x00invalid"), 0o644))

	_, err := LoadProfile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing profile")
}

func TestLoadProfile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	_, err := LoadProfile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadProfile_RealProfiles(t *testing.T) {
	profiles := []struct {
		path       string
		components int
	}{
		{"../../profiles/rhoai/profile.yaml", 10},
		{"../../profiles/odh/profile.yaml", 5},
		{"../../profiles/cert-manager/profile.yaml", 3},
		{"../../profiles/rh-kueue/profile.yaml", 1},
	}

	for _, tt := range profiles {
		t.Run(filepath.Base(filepath.Dir(tt.path)), func(t *testing.T) {
			if _, err := os.Stat(tt.path); os.IsNotExist(err) {
				t.Skip("profile not found")
			}
			p, err := LoadProfile(tt.path)
			require.NoError(t, err)
			assert.Len(t, p.Components, tt.components)
			assert.Empty(t, p.Warnings, "real profiles should have no unknown field warnings")
		})
	}
}
