package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opendatahub-io/operator-chaos/pkg/experiment"
	"github.com/opendatahub-io/operator-chaos/pkg/generate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateExperiments_RHOAIProfile(t *testing.T) {
	profilePath := "../../profiles/rhoai/profile.yaml"
	tmplDir := "../../templates"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skip("RHOAI profile not found")
	}
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		t.Skip("templates directory not found")
	}

	outDir := t.TempDir()

	opts := generate.GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := generate.Generate(opts)
	require.NoError(t, err)
	assert.Greater(t, result.Generated, 0, "should generate at least some experiments")

	filesValidated := 0
	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return nil
		}

		filesValidated++

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("failed to read %s: %v", path, readErr)
			return nil
		}
		if strings.Contains(string(data), "${") {
			t.Errorf("unresolved variable placeholder in %s", path)
		}

		exp, loadErr := experiment.Load(path)
		if loadErr != nil {
			t.Errorf("failed to load generated experiment %s: %v", path, loadErr)
			return nil
		}

		errs := experiment.Validate(exp)
		if len(errs) > 0 {
			t.Errorf("validation errors in %s: %v", path, errs)
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, result.Generated+result.Copied, filesValidated,
		"number of files on disk should match Generated+Copied from result")
}

func TestGenerateExperiments_ODHProfile(t *testing.T) {
	profilePath := "../../profiles/odh/profile.yaml"
	tmplDir := "../../templates"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skip("ODH profile not found")
	}
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		t.Skip("templates directory not found")
	}

	outDir := t.TempDir()

	opts := generate.GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := generate.Generate(opts)
	require.NoError(t, err)
	assert.Greater(t, result.Generated, 0)

	filesValidated := 0
	err = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		filesValidated++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("failed to read %s: %v", path, readErr)
			return nil
		}
		if strings.Contains(string(data), "${") {
			t.Errorf("unresolved variable placeholder in %s", path)
		}
		exp, loadErr := experiment.Load(path)
		if loadErr != nil {
			t.Errorf("failed to load %s: %v", path, loadErr)
			return nil
		}
		errs := experiment.Validate(exp)
		if len(errs) > 0 {
			t.Errorf("validation errors in %s: %v", path, errs)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, result.Generated+result.Copied, filesValidated,
		"number of files on disk should match Generated+Copied from result")
}

func TestGenerateExperiments_CertManagerProfile(t *testing.T) {
	profilePath := "../../profiles/cert-manager/profile.yaml"
	tmplDir := "../../templates"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skip("cert-manager profile not found")
	}
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		t.Skip("templates directory not found")
	}

	outDir := t.TempDir()

	opts := generate.GenerateOptions{
		ProfilePath: profilePath,
		TemplateDir: tmplDir,
		OutputDir:   outDir,
	}

	result, err := generate.Generate(opts)
	require.NoError(t, err)

	_, routeErr := os.Stat(filepath.Join(outDir, "controller", "route-backend-disruption.yaml"))
	assert.True(t, os.IsNotExist(routeErr), "route templates should not be generated for kubernetes platform")

	assert.FileExists(t, filepath.Join(outDir, "controller", "pod-kill.yaml"))

	assert.Greater(t, result.Generated, 0)

	filesOnDisk := 0
	_ = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			filesOnDisk++
		}
		return nil
	})
	assert.Equal(t, result.Generated+result.Copied, filesOnDisk,
		"number of files on disk should match Generated+Copied from result")
}

func TestResolveProfileYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	profileDir := filepath.Join(tmp, "profiles", "test")
	require.NoError(t, os.MkdirAll(profileDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "profile.yaml"), []byte("name: test\n"), 0o644))

	path, err := resolveProfileYAML("test")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("profiles", "test", "profile.yaml"), path)
}

func TestResolveProfileYAML_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	_, err := resolveProfileYAML("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveProfileYAML_PathTraversal(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"parent directory", "../evil"},
		{"bare dot-dot", ".."},
		{"bare dot", "."},
		{"slash only", "/"},
		{"contains slash", "foo/bar"},
		{"contains backslash", "foo\\bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveProfileYAML(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "path separators")
		})
	}
}
