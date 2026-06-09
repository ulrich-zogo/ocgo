package app

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/daemon"
	"ocgo/internal/models"
	"ocgo/internal/process"
)

func CodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Manage Codex integrations",
	}
	desktop := &cobra.Command{
		Use:   "desktop",
		Short: "Manage Codex Desktop integration",
	}
	desktop.AddCommand(codexDesktopStatusCmd(), codexDesktopEnableCmd())
	cmd.AddCommand(desktop)
	return cmd
}

func codexDesktopStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Codex Desktop provider status",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			mgr := codex.NewManager()
			rep, err := mgr.DesktopStatus()
			if err != nil {
				return err
			}
			if !rep.Managed {
				fmt.Fprintln(out, "Codex Desktop is not managed by OCGO")
				return nil
			}
			fmt.Fprintf(out, "Codex Desktop mode: %s\n", rep.Mode)
			if rep.Mode == codex.DesktopModeOpenCode {
				if rep.BaseURL != "" {
					fmt.Fprintf(out, "Base URL: %s\n", rep.BaseURL)
				}
				if rep.Model != "" {
					fmt.Fprintf(out, "Model: %s\n", rep.Model)
				}
				cfg, cfgErr := config.LoadConfig()
				if cfgErr == nil {
					ds := daemon.NewManager()
					s, sErr := ds.Status(cfg)
					daemonStatus := "not running"
					if sErr == nil {
						if s.Running {
							daemonStatus = "running"
						}
					}
					fmt.Fprintf(out, "Daemon: %s\n", daemonStatus)
				} else {
					fmt.Fprintln(out, "Daemon: unknown (config not loaded)")
				}
				if rep.BackupFile != "" {
					fmt.Fprintf(out, "Backup: %s\n", rep.BackupFile)
				}
			}
			return nil
		},
	}
}

func codexDesktopEnableCmd() *cobra.Command {
	var modelFlag string
	cmd := &cobra.Command{
		Use:   "enable [opencode|chatgpt]",
		Short: "Switch Codex Desktop between OCGO/OpenCode Go and ChatGPT/OpenAI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "opencode":
				return runCodexDesktopEnableOpenCode(cmd, modelFlag, out)
			case "chatgpt":
				return runCodexDesktopEnableChatGPT(out)
			default:
				return fmt.Errorf("unknown mode %q (expected 'opencode' or 'chatgpt')", args[0])
			}
		},
	}
	cmd.Flags().StringVar(&modelFlag, "model", "", "OpenCode Go model ID (only for 'enable opencode')")
	return cmd
}

func runCodexDesktopEnableOpenCode(cmd *cobra.Command, modelFlag string, out io.Writer) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	ds := daemon.NewManager()
	if _, _, err := ds.Start(cfg); err != nil {
		return fmt.Errorf("start OCGO daemon: %w", err)
	}
	resolved, err := models.ResolveEffectiveModel(modelFlag)
	if err != nil {
		return err
	}
	base := process.BaseURL(cfg)
	mgr := codex.NewManager()
	st, err := mgr.EnableDesktopOpenCode(base, resolved)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Codex Desktop enabled with OCGO/OpenCode Go")
	fmt.Fprintf(out, "Base URL: %s\n", st.BaseURL)
	fmt.Fprintf(out, "Model: %s\n", st.Model)
	if st.BackupFile != "" {
		fmt.Fprintf(out, "Backup: %s\n", st.BackupFile)
	}
	return nil
}

func runCodexDesktopEnableChatGPT(out io.Writer) error {
	mgr := codex.NewManager()
	st, err := mgr.EnableDesktopChatGPT()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Codex Desktop restored to ChatGPT/OpenAI configuration")
	if st.BackupFile != "" {
		fmt.Fprintf(out, "Restored: %s\n", st.BackupFile)
	}
	return nil
}
