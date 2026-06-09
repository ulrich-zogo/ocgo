package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/models"
)

func OpencodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "OpenCode Go specific commands (model selection, future desktop)",
	}
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Manage the OpenCode Go default model selection",
	}
	currentCmd := &cobra.Command{
		Use:   "current",
		Short: "Print the OpenCode Go model that will be used by launch",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, err := models.GetDefaultModel()
			configured := err == nil && selected != ""
			if !configured {
				// Fall back to the first known model.
				known := models.KnownIDs()
				if len(known) == 0 {
					return fmt.Errorf("no known OpenCode Go models available")
				}
				selected = known[0]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default OpenCode Go model: %s\n", selected)
			fmt.Fprintf(cmd.OutOrStdout(), "Source: %s\n", DescribeSelectionSource(configured))
			return nil
		},
	}
	setDefaultCmd := &cobra.Command{
		Use:   "set-default <model>",
		Short: "Set the OpenCode Go default model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := models.SetDefaultModel(args[0]); err != nil {
				return err
			}
			id, _ := models.GetDefaultModel()
			fmt.Fprintf(cmd.OutOrStdout(), "Default OpenCode Go model set to %s\n", id)
			return nil
		},
	}
	modelCmd.AddCommand(currentCmd, setDefaultCmd)
	cmd.AddCommand(modelCmd)
	return cmd
}
