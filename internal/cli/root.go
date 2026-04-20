package cli

import (
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

// NewRootCommand builds the top-level cobra command for the operator-chaos CLI.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator-chaos",
		Short: "Chaos engineering framework for Kubernetes operators",
		Long: `Operator Chaos tests operator reconciliation semantics.
It validates that operators recover managed resources correctly after
fault injection, not just that pods restart.`,
	}

	cmd.PersistentFlags().String("kubeconfig", "", "path to kubeconfig file")
	cmd.PersistentFlags().String("namespace", v1alpha1.DefaultNamespace, "target namespace")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")

	cmd.AddCommand(
		newValidateCommand(),
		newRunCommand(),
		newCleanCommand(),
		newInitCommand(),
		newAnalyzeCommand(),
		newSuiteCommand(),
		newReportCommand(),
		newVersionCommand(),
		newTypesCommand(),
		newPreflightCommand(),
		newControllerCommand(),
		newGenerateCommand(),
		newDiffCommand(),
		newDiffCRDsCommand(),
		newValidateVersionCommand(),
		newSimulateUpgradeCommand(),
		newUpgradeCommand(),
		newPlaybookCommand(),
	)

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("operator-chaos version " + Version)
		},
	}
}
