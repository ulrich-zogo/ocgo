package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/process"
)

func StopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop background proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := config.ReadPID()
			if err != nil {
				cfg, cfgErr := config.LoadConfig()
				if cfgErr != nil {
					return errors.New("proxy is not running")
				}
				pid, err = process.FindListenerPID(cfg.Port)
				if err != nil {
					return errors.New("proxy is not running")
				}
			}
			p, err := os.FindProcess(pid)
			if err != nil {
				return err
			}
			_ = os.Remove(config.PIDFile())
			if err := p.Kill(); err != nil {
				return err
			}
			fmt.Printf("Stopped proxy process %d\n", pid)
			return nil
		},
	}
}
