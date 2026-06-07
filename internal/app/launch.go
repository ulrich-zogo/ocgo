package app

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
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
			if model != "" {
				c.Env = append(c.Env,
					"ANTHROPIC_MODEL="+model,
					"ANTHROPIC_SMALL_FAST_MODEL="+model,
					"ANTHROPIC_CUSTOM_MODEL_OPTION="+model,
					"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME="+model+" via OCGO",
					"ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION=OpenCode Go model routed through OCGO",
				)
			} else {
				opus := mapping.ResolveMappedModel("claude", "claude-opus", mappings)
				sonnet := mapping.ResolveMappedModel("claude", "claude-sonnet", mappings)
				haiku := mapping.ResolveMappedModel("claude", "claude-haiku", mappings)
				if opus != "claude-opus" {
					c.Env = append(c.Env,
						"ANTHROPIC_DEFAULT_OPUS_MODEL="+opus,
						"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME="+opus+" via OCGO",
						"ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
					)
				}
				if sonnet != "claude-sonnet" {
					c.Env = append(c.Env,
						"ANTHROPIC_DEFAULT_SONNET_MODEL="+sonnet,
						"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME="+sonnet+" via OCGO",
						"ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
					)
				}
				if haiku != "claude-haiku" {
					c.Env = append(c.Env,
						"ANTHROPIC_DEFAULT_HAIKU_MODEL="+haiku,
						"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME="+haiku+" via OCGO",
						"ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
						"ANTHROPIC_SMALL_FAST_MODEL="+haiku,
					)
				}
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
			if err := codex.EnsureConfig(base); err != nil {
				return fmt.Errorf("failed to configure codex: %w", err)
			}
			if codexConfigOnly {
				fmt.Printf("Configured Codex profile %q in %s\n", config.CodexProfileName, config.CodexProfileConfigFile())
				return nil
			}
			if err := codex.CheckVersion(); err != nil {
				return err
			}
			serverCmd, err := process.StartLaunchServer(base)
			if err != nil {
				return err
			}
			if serverCmd != nil {
				defer process.StopManagedServer(serverCmd)
			}
			codexArgs := []string{"--profile", config.CodexProfileName}
			if model != "" {
				codexArgs = append(codexArgs, "-m", model)
			}
			codexArgs = append(codexArgs, args...)
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
