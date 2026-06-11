package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/configlifecycle"
)

func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage OCGO configuration files safely",
	}
	cmd.AddCommand(configPathsCmd(), configInspectCmd(), configBackupCmd(), configRestoreCmd(), configResetCmd())
	return cmd
}

func configPathsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Show OCGO configuration paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := configlifecycle.AllPaths()
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}
			fmt.Fprint(cmd.OutOrStdout(), "OCGO configuration paths:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  ocgo config dir:      %s\n", p.ConfigDir)
			fmt.Fprintf(cmd.OutOrStdout(), "  ocgo config file:     %s\n", p.ConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  model mapping:        %s\n", p.ModelMappingFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  model selection:      %s\n", p.ModelSelectionFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  model cache:          %s\n", p.ModelCacheFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  daemon state:         %s\n", p.DaemonStateFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  desktop state:        %s\n", p.DesktopStateFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  daemon pid:           %s\n", p.DaemonPIDFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  daemon log:           %s\n", p.DaemonLogFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  codex config:         %s\n", p.CodexConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  codex ocgo profile:   %s\n", p.CodexProfileFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  codex model catalog:  %s\n", p.CodexCatalogFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  codex backups:        %s\n", p.CodexBackupsDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print paths as JSON")
	return cmd
}

func configInspectCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect OCGO configuration state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ins := configlifecycle.Inspect()
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(ins)
			}
			fmt.Fprint(cmd.OutOrStdout(), "OCGO configuration inspection:\n")
			fmt.Fprintln(cmd.OutOrStdout(), "\nCore:")
			fmt.Fprintf(cmd.OutOrStdout(), "  config file:          %s\n", ins.Core.ConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  host:                 %s\n", ins.Core.Host)
			fmt.Fprintf(cmd.OutOrStdout(), "  port:                 %d\n", ins.Core.Port)
			fmt.Fprintf(cmd.OutOrStdout(), "  OpenCode API key:     %s\n", ins.Core.OpenCodeAPIKey)
			fmt.Fprintln(cmd.OutOrStdout(), "\nModel:")
			fmt.Fprintf(cmd.OutOrStdout(), "  selected model:       %s\n", ins.Model.SelectedModel)
			fmt.Fprintf(cmd.OutOrStdout(), "  mapping file:         %s\n", ins.Model.MappingFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  model cache:          %s\n", ins.Model.ModelCache)
			fmt.Fprintln(cmd.OutOrStdout(), "\nDaemon:")
			fmt.Fprintf(cmd.OutOrStdout(), "  state file:           %s\n", ins.Daemon.StateFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  pid file:             %s\n", ins.Daemon.PIDFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  pid status:           %s\n", ins.Daemon.PIDStatus)
			fmt.Fprintf(cmd.OutOrStdout(), "  log file:             %s\n", ins.Daemon.LogFile)
			fmt.Fprintln(cmd.OutOrStdout(), "\nCodex CLI:")
			fmt.Fprintf(cmd.OutOrStdout(), "  config file:          %s\n", ins.CodexCLI.ConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  ocgo profile:         %s\n", ins.CodexCLI.OcgoProfile)
			fmt.Fprintf(cmd.OutOrStdout(), "  model catalog:        %s\n", ins.CodexCLI.ModelCatalog)
			fmt.Fprintln(cmd.OutOrStdout(), "\nCodex Desktop:")
			fmt.Fprintf(cmd.OutOrStdout(), "  state file:           %s\n", ins.CodexDesktop.StateFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  active provider:      %s\n", ins.CodexDesktop.ActiveProvider)
			fmt.Fprintf(cmd.OutOrStdout(), "  backup file:          %s\n", ins.CodexDesktop.BackupFile)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print inspection as JSON")
	return cmd
}

func configBackupCmd() *cobra.Command {
	var output string
	var includeCodexConfig bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup OCGO configuration files",
		RunE: func(cmd *cobra.Command, args []string) error {
			dest := output
			if dest == "" {
				bDir := filepath.Join(config.ConfigDir(), "backups")
				os.MkdirAll(bDir, 0755)
				dest = filepath.Join(bDir, fmt.Sprintf("ocgo-config-backup-%s.zip", time.Now().UTC().Format("20060102-150405")))
			}
			result, err := configlifecycle.Backup(dest, includeCodexConfig)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Backup created: %s\n", result.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Files included: %d\n", result.FileCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Output path for the backup zip")
	cmd.Flags().BoolVar(&includeCodexConfig, "include-codex-config", false, "Include .codex/config.toml in backup")
	return cmd
}

func configRestoreCmd() *cobra.Command {
	var dryRun, yes, includeCodexConfig bool
	cmd := &cobra.Command{
		Use:   "restore <backup.zip>",
		Short: "Restore OCGO configuration from a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := configlifecycle.Restore(args[0], configlifecycle.RestoreOptions{
				DryRun: dryRun, Yes: yes, IncludeCodexConfig: includeCodexConfig,
			})
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Backup created at: (from manifest)\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Files to restore (%d):\n", len(result.Files))
				for _, f := range result.Files {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %d files.\n", len(result.Files))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be restored without modifying anything")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&includeCodexConfig, "include-codex-config", false, "Allow restoring .codex/config.toml if present in backup")
	return cmd
}

func configResetCmd() *cobra.Command {
	var scopeStr string
	var dryRun, yes, includeBackups, noBackup bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset OCGO configuration files",
		Long: `Reset OCGO-managed configuration files by scope.

Scopes:
  ocgo            Reset OCGO core config, mapping, selection, daemon, desktop state
  cache           Reset model catalog cache only
  codex-cli       Reset Codex CLI OCGO profile and model catalog
  codex-desktop   Reset Codex Desktop OCGO state (restores desktop backup if available)
  all             Reset all of the above

By default --scope ocgo is used. A backup is created automatically before any destructive
operation unless --no-backup is specified (requires --yes).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := configlifecycle.ResetScope(strings.ReplaceAll(scopeStr, "_", "-"))
			if scope == "" {
				scope = configlifecycle.ResetScopeOcgo
			}
			result, err := configlifecycle.Reset(configlifecycle.ResetOptions{
				Scope: scope, DryRun: dryRun, Yes: yes,
				IncludeBackups: includeBackups, NoBackup: noBackup,
			})
			if err != nil {
				return err
			}
			if scope == configlifecycle.ResetScopeCodexDesktop {
				fmt.Fprintln(cmd.OutOrStdout(), "Codex Desktop state reset.")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'ocgo codex desktop enable chatgpt' to restore the original ChatGPT/OpenAI provider.")
				return nil
			}
			if len(result.Removed) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Nothing to reset for scope %s.\n", scope)
				return nil
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Scope: %s\n", scope)
				fmt.Fprintf(cmd.OutOrStdout(), "Files to remove (%d):\n", len(result.Removed))
				for _, f := range result.Removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scope: %s\n", scope)
			if result.Backup != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Backup: %s\n", result.Backup)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d files.\n", len(result.Removed))
			return nil
		},
	}
	cmd.Flags().StringVar(&scopeStr, "scope", "ocgo", "Reset scope: ocgo, cache, codex-cli, codex-desktop, all")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without modifying anything")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&includeBackups, "include-backups", false, "Also remove backup files (requires --scope all)")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Skip automatic backup before reset (requires --yes)")
	return cmd
}
