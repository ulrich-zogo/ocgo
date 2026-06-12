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
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show OCGO daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			mgr := daemon.NewManager()

			if jsonOut {
				ds := mgr.DetailedStatus(cfg)
				return writeJSON(cmd.OutOrStdout(), ds)
			}

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
			ds := mgr.DetailedStatus(cfg)
			fmt.Fprintln(out, "OCGO daemon status:")
			fmt.Fprintf(out, "  state file:     %s\n", ds.StateFileStatus)
			fmt.Fprintf(out, "  pid file:       %s\n", ds.PIDFileStatus)
			fmt.Fprintf(out, "  pid:            %d\n", ds.PID)
			fmt.Fprintf(out, "  process:        %s\n", ds.Process)
			fmt.Fprintf(out, "  health:         %s\n", ds.Health)
			fmt.Fprintf(out, "  base url:       %s\n", ds.BaseURL)
			fmt.Fprintf(out, "  log file:       %s\n", ds.LogFile)
			fmt.Fprintf(out, "  started at:     %s\n", ds.StartedAt)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print daemon status as JSON")
	return cmd
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
