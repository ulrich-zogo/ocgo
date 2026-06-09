package app

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
	"ocgo/internal/process"
)

func LaunchCmd() *cobra.Command {
	var model string
	var yes bool
	var codexConfigOnly bool
	cmd := &cobra.Command{Use: "launch", Short: "Launch tools through ocgo"}
	claude := &cobra.Command{
		Use: "claude [-- claude args...]", Short: "Launch Claude Code through OpenCode Go", Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			base := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
			serverCmd, err := process.StartLaunchServer(base)
			if err != nil {
				return err
			}
			if serverCmd != nil {
				defer process.StopManagedServer(serverCmd)
			}
			claudeArgs := append([]string{}, args...)
			if yes {
				claudeArgs = append([]string{"--dangerously-skip-permissions"}, claudeArgs...)
			}
			bin, err := exec.LookPath("claude")
			if err != nil {
				return fmt.Errorf("claude not found in PATH: %w", err)
			}
			c := exec.Command(bin, claudeArgs...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			c.Env = append(os.Environ(), "ANTHROPIC_BASE_URL="+base, "ANTHROPIC_AUTH_TOKEN=unused")
			mappings, err := mapping.LoadModelMappings()
			if err != nil {
				return err
			}
			modelEnv, hasEffective, err := BuildClaudeModelEnv(model)
			if err != nil {
				return err
			}
			if hasEffective {
				c.Env = append(c.Env, modelEnv...)
			} else {
				c.Env = append(c.Env, BuildClaudeLegacyMappingEnv(mappings)...)
			}
			mapping.PrintLaunchMapping("claude", mappings["claude"])
			return c.Run()
		},
	}
	claude.Flags().StringVar(&model, "model", "", "OpenCode Go model ID")
	claude.Flags().BoolVar(&yes, "yes", false, "Allow Claude Code to skip permission prompts")

	codexCmd := &cobra.Command{
		Use: "codex [-- codex args...]", Short: "Launch Codex CLI through OpenCode Go", Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			base := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
			mgr := codex.NewManager()
			if err := mgr.EnsureCLIConfig(base); err != nil {
				return fmt.Errorf("failed to configure codex: %w", err)
			}
			selectedModel, err := models.ResolveEffectiveModel(model)
			if err != nil {
				return err
			}
			if codexConfigOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "Configured Codex profile %q in %s\n", config.CodexProfileName, config.CodexProfileConfigFile())
				fmt.Fprintf(cmd.OutOrStdout(), "Effective OpenCode Go model: %s\n", selectedModel)
				return nil
			}
			if err := mgr.CheckVersion(); err != nil {
				return err
			}
			serverCmd, err := process.StartLaunchServer(base)
			if err != nil {
				return err
			}
			if serverCmd != nil {
				defer process.StopManagedServer(serverCmd)
			}
			codexArgs := BuildCodexArgsWithResolvedModel(selectedModel, args)
			bin, err := exec.LookPath("codex")
			if err != nil {
				return fmt.Errorf("codex not found in PATH; install with: npm install -g @openai/codex: %w", err)
			}
			c := exec.Command(bin, codexArgs...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			c.Env = append(os.Environ(), "OPENAI_API_KEY=ocgo")
			if mappings, err := mapping.LoadModelMappings(); err == nil {
				mapping.PrintLaunchMapping("codex", mappings["codex"])
			}
			return c.Run()
		},
	}
	codexCmd.Flags().StringVar(&model, "model", "", "OpenCode Go model ID")
	codexCmd.Flags().BoolVar(&codexConfigOnly, "config", false, "Configure Codex profile without launching")
	cmd.AddCommand(claude, codexCmd)
	return cmd
}
