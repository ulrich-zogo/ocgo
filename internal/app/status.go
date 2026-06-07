package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/process"
)

func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show proxy status",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Proxy is not running")
				return
			}
			base := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
			if !process.Healthy(base) {
				fmt.Println("Proxy is not running")
				return
			}
			if pid, err := config.ReadPID(); err == nil {
				fmt.Printf("Proxy is running on %s:%d (PID %d)\n", cfg.Host, cfg.Port, pid)
				return
			}
			if pid, err := process.FindListenerPID(cfg.Port); err == nil {
				fmt.Printf("Proxy is running on %s:%d (PID %d, discovered from listener)\n", cfg.Host, cfg.Port, pid)
				return
			}
			fmt.Printf("Proxy is running on %s:%d (no ocgo PID file)\n", cfg.Host, cfg.Port)
		},
	}
}
