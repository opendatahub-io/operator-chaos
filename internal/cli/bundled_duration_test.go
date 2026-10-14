package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opendatahub-io/operator-chaos/pkg/experiment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBundledExperimentsFitDefaultTimeout verifies every experiment bundled
// in experiments/ passes validateRunDuration at the default --timeout of
// 21m, so the CLI works out of the box without a per-experiment override.
func TestBundledExperimentsFitDefaultTimeout(t *testing.T) {
	var files []string
	require.NoError(t, filepath.Walk("../../experiments", func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		// fixtures/ holds supporting objects (ClusterQueues etc.), not experiments
		if !info.IsDir() && filepath.Ext(path) == ".yaml" && filepath.Base(filepath.Dir(path)) != "fixtures" {
			files = append(files, path)
		}
		return nil
	}))
	require.NotEmpty(t, files)

	for _, file := range files {
		exp, err := experiment.Load(file)
		require.NoError(t, err, file)
		assert.NoError(t, validateRunDuration(exp, 21*time.Minute), file)
	}
}
