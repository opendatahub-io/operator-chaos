package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectExperimentFiles_Flat(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("---"), 0o644))

	files, err := collectExperimentFiles(dir, false)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, filepath.Join(dir, "a.yaml"), files[0])
	assert.Equal(t, filepath.Join(dir, "b.yaml"), files[1])
}

func TestCollectExperimentFiles_Recursive(t *testing.T) {
	dir := t.TempDir()
	compA := filepath.Join(dir, "alpha")
	compB := filepath.Join(dir, "beta")
	require.NoError(t, os.MkdirAll(compA, 0o750))
	require.NoError(t, os.MkdirAll(compB, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(compA, "pod-kill.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(compA, "rbac.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(compB, "pod-kill.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "global.yaml"), []byte("---"), 0o644))

	files, err := collectExperimentFiles(dir, true)
	require.NoError(t, err)

	expected := []string{
		filepath.Join(compA, "pod-kill.yaml"),
		filepath.Join(compA, "rbac.yaml"),
		filepath.Join(compB, "pod-kill.yaml"),
		filepath.Join(dir, "global.yaml"),
	}
	assert.Equal(t, expected, files)
}

func TestCollectExperimentFiles_RecursiveSortOrder(t *testing.T) {
	dir := t.TempDir()
	compZ := filepath.Join(dir, "z-comp")
	require.NoError(t, os.MkdirAll(compZ, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(compZ, "a.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b-top.yaml"), []byte("---"), 0o644))

	files, err := collectExperimentFiles(dir, true)
	require.NoError(t, err)

	assert.Len(t, files, 2)
	assert.Equal(t, filepath.Join(dir, "b-top.yaml"), files[0], "top-level 'b' sorts before subdir 'z'")
	assert.Equal(t, filepath.Join(compZ, "a.yaml"), files[1])
}

func TestCollectExperimentFiles_DeeplyNestedIgnored(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "comp", "subdir")
	require.NoError(t, os.MkdirAll(deep, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(deep, "deep.yaml"), []byte("---"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "comp", "top.yaml"), []byte("---"), 0o644))

	files, err := collectExperimentFiles(dir, true)
	require.NoError(t, err)

	assert.Len(t, files, 1, "deeply nested file should be ignored")
	assert.Contains(t, files[0], "top.yaml")
}

func TestCollectExperimentFiles_RecursiveEmpty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "empty"), 0o750))

	_, err := collectExperimentFiles(dir, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no experiment files found")
}
