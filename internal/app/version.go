package app

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/buildinfo"
)

func VersionCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print OCGO version and build metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := buildinfo.Current()

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ocgo version %s\n", info.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", info.Commit)
			fmt.Fprintf(cmd.OutOrStdout(), "built: %s\n", info.Date)
			fmt.Fprintf(cmd.OutOrStdout(), "go: %s\n", info.GoVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "platform: %s/%s\n", info.OS, info.Arch)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print version information as JSON")
	return cmd
}
