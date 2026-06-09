package app

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/daemon"
)

func DaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the OCGO local daemon (Desktop-first)",
	}
	cmd.AddCommand(daemonStartCmd(), daemonStatusCmd(), daemonStopCmd(), daemonRestartCmd())
	return cmd
}

func daemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the OCGO daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			mgr := daemon.NewManager()
			st, alreadyRunning, err := mgr.Start(cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if alreadyRunning {
				fmt.Fprintln(out, "OCGO daemon already running")
			} else {
				fmt.Fprintln(out, "OCGO daemon started")
			}
			fmt.Fprintf(out, "Base URL: %s\n", st.BaseURL)
			if st.PID > 0 {
				fmt.Fprintf(out, "PID: %d\n", st.PID)
			}
			fmt.Fprintf(out, "Log: %s\n", config.LogFile())
			return nil
		},
	}
}

func daemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show OCGO daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			mgr := daemon.NewManager()
			s, err := mgr.Status(cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !s.Running {
				fmt.Fprintln(out, "OCGO daemon is not running")
				fmt.Fprintf(out, "Base URL: %s\n", s.BaseURL)
				return nil
			}
			if !s.HasState {
				fmt.Fprintln(out, "OCGO proxy is running, but daemon state is missing")
				fmt.Fprintf(out, "Base URL: %s\n", s.BaseURL)
				if s.PID > 0 {
					fmt.Fprintf(out, "PID: %d\n", s.PID)
				}
				return nil
			}
			fmt.Fprintln(out, "OCGO daemon is running")
			fmt.Fprintf(out, "Base URL: %s\n", s.BaseURL)
			if s.PID > 0 {
				fmt.Fprintf(out, "PID: %d\n", s.PID)
			}
			fmt.Fprintf(out, "State: %s\n", mgr.StateFile)
			fmt.Fprintf(out, "Log: %s\n", config.LogFile())
			return nil
		},
	}
}

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the OCGO daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			mgr := daemon.NewManager()
			out := cmd.OutOrStdout()
			if err := mgr.Stop(cfg); err != nil {
				if errors.Is(err, daemon.ErrNotRunning) {
					fmt.Fprintln(out, "OCGO daemon is not running")
					return nil
				}
				return err
			}
			fmt.Fprintln(out, "OCGO daemon stopped")
			return nil
		},
	}
}

func daemonRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the OCGO daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			mgr := daemon.NewManager()
			if err := mgr.Stop(cfg); err != nil && !errors.Is(err, daemon.ErrNotRunning) {
				return err
			}
			st, _, err := mgr.Start(cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "OCGO daemon restarted")
			fmt.Fprintf(out, "Base URL: %s\n", st.BaseURL)
			if st.PID > 0 {
				fmt.Fprintf(out, "PID: %d\n", st.PID)
			}
			return nil
		},
	}
}
