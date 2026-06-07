package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/models"
)

func ListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "models"},
		Short:   "List OpenCode Go models",
		Run: func(cmd *cobra.Command, args []string) {
			models.RefreshAll()
			fmt.Println("OpenCode Go models:")
			for _, m := range models.KnownIDs() {
				fmt.Printf("  %s\n", m)
			}
		},
	}
}
