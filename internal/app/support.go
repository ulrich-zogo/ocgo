package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/support"
)

func SupportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "support",
		Short: "Generate diagnostic support bundles",
	}
	cmd.AddCommand(supportBundleCmd())
	return cmd
}

func supportBundleCmd() *cobra.Command {
	var (
		output   string
		force    bool
		noLogs   bool
		jsonOut  bool
	)
	c := &cobra.Command{
		Use:   "bundle",
		Short: "Generate a redacted diagnostic support bundle",
		Long: `Generate a redacted ZIP archive containing OCGO diagnostics.

The bundle includes version info, doctor report, daemon status,
config paths, config inspection, environment metadata, state files,
and redacted logs. It is safe to attach to bug reports.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := support.BundleOptions{
				OutputPath:  output,
				Force:       force,
				IncludeLogs: !noLogs,
			}
			result, err := support.CreateBundle(opts)
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "OCGO support bundle created:")
			fmt.Fprintf(out, "  path: %s\n", result.Path)
			fmt.Fprintf(out, "  files: %d\n", len(result.Files))
			fmt.Fprintf(out, "  redacted: %v\n", result.Redacted)
			fmt.Fprintf(out, "  logs included: %v\n", result.LogsIncluded)
			return nil
		},
	}
	c.Flags().StringVar(&output, "output", "", "Output path for the bundle zip")
	c.Flags().BoolVar(&force, "force", false, "Overwrite existing output file")
	c.Flags().BoolVar(&noLogs, "no-logs", false, "Exclude log files from the bundle")
	c.Flags().BoolVar(&jsonOut, "json", false, "Print result as JSON only")
	return c
}
