package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
)

func SetupCmd() *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Save your OpenCode Go API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(key) == "" {
				key = os.Getenv("OCGO_API_KEY")
			}
			if strings.TrimSpace(key) == "" {
				fmt.Print("OpenCode Go API key: ")
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil && line == "" {
					return err
				}
				key = line
			}
			cfg := config.Config{APIKey: strings.TrimSpace(key), Host: config.DefaultHost, Port: config.DefaultPort}
			if cfg.APIKey == "" {
				return errors.New("API key cannot be empty")
			}
			return config.SaveConfig(cfg)
		},
	}
	cmd.Flags().StringVar(&key, "api-key", "", "OpenCode Go API key")
	return cmd
}
