package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opendatahub-io/operator-chaos/pkg/generate"
	"github.com/spf13/cobra"
)

func newGenerateExperimentsCommand() *cobra.Command {
	var (
		profile      string
		outputDir    string
		component    string
		templateName string
		setVars      []string
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "experiments",
		Short: "Generate experiment YAML from templates and a profile",
		Long: `Resolve parameterized experiment templates against a named profile to produce
concrete experiment YAML files. Each template uses ${VAR} placeholders that
are replaced with values from the profile's per-component definitions.

Templates are read from the templates/ directory. Output is organized as
<output-dir>/<component>/<template-name>.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dryRun && outputDir == "" {
				return fmt.Errorf("--output is required (unless --dry-run)")
			}

			profilePath, err := resolveProfileYAML(profile)
			if err != nil {
				return err
			}

			tmplDir := "templates"
			if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
				return fmt.Errorf("templates directory not found: %s", tmplDir)
			}

			opts := generate.GenerateOptions{
				ProfilePath:  profilePath,
				TemplateDir:  tmplDir,
				OutputDir:    outputDir,
				Component:    component,
				TemplateName: templateName,
				SetVars:      setVars,
				DryRun:       dryRun,
			}

			result, err := generate.Generate(opts)
			if err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run: would generate %d experiments for %d components, copy %d profile-specific, skip %d\n",
					result.Generated, result.Components, result.Copied, result.Skipped)
				for _, p := range result.Plan {
					fmt.Fprintf(os.Stderr, "  %s/%s (%s)\n", p.Component, p.Template, p.Source)
				}
				return nil
			}

			fmt.Fprintf(os.Stderr, "Generated %d experiments for %d components, copied %d profile-specific, skipped %d\n",
				result.Generated, result.Components, result.Copied, result.Skipped)
			fmt.Fprintf(os.Stderr, "Output: %s\n", outputDir)

			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "profile name (resolves to profiles/<name>/profile.yaml)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory for generated experiments")
	cmd.Flags().StringVar(&component, "component", "", "generate for a single component only")
	cmd.Flags().StringVar(&templateName, "template", "", "generate from a single template only")
	cmd.Flags().StringArrayVar(&setVars, "set-var", nil, "override profile variable (component:field=value, repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be generated without writing files")
	_ = cmd.MarkFlagRequired("profile")

	return cmd
}

func resolveProfileYAML(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("profile name is required")
	}

	if name == "." || name == ".." || strings.ContainsAny(name, "/\\") || filepath.Base(name) != name {
		return "", fmt.Errorf("profile name %q must not contain path separators", name)
	}

	path := filepath.Join("profiles", name, "profile.yaml")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("profile %q not found at %s: %w", name, path, err)
	}

	return path, nil
}
