package app

import (
	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/process"
	"ocgo/internal/proxy"
)

func ServeCmd() *cobra.Command {
	var background bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start local Anthropic-compatible proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if background {
				return process.StartBackground()
			}
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			return proxy.RunServer(cfg)
		},
	}
	cmd.Flags().BoolVarP(&background, "background", "b", false, "Run proxy in the background")
	return cmd
}
