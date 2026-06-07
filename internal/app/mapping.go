package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

func MappingCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mapping", Short: "Manage tool model mappings to OpenCode Go models"}
	cmd.AddCommand(toolMappingCmd("claude"), toolMappingCmd("codex"))
	return cmd
}

func toolMappingCmd(tool string) *cobra.Command {
	cmd := &cobra.Command{Use: tool, Short: fmt.Sprintf("Manage %s model mappings", tool)}
	cmd.AddCommand(
		&cobra.Command{
			Use: "show", Short: "Show current mapping",
			RunE: func(cmd *cobra.Command, args []string) error {
				m, err := mapping.LoadModelMappings()
				if err != nil {
					return err
				}
				mapping.PrintToolMapping(tool, m[tool])
				return nil
			},
		},
		&cobra.Command{
			Use: "get <source-model>", Short: "Get one mapped OpenCode Go model", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				m, err := mapping.LoadModelMappings()
				if err != nil {
					return err
				}
				source := strings.TrimSpace(args[0])
				normalized := models.NormalizeID(source)
				if target := mapping.ResolveMappedModel(tool, source, m); target != normalized {
					fmt.Printf("%s -> %s\n", source, target)
					return nil
				}
				return fmt.Errorf("no mapping for %q in %s", source, tool)
			},
		},
		&cobra.Command{
			Use: "set <source-model> <opencode-model>", Short: "Set one model mapping", Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				source := strings.TrimSpace(args[0])
				target := strings.TrimSpace(args[1])
				if source == "" || target == "" {
					return errors.New("source and target models cannot be empty")
				}
				if !mapping.KnownOpenCodeModel(target) {
					return fmt.Errorf("unknown OpenCode Go model %q; run `ocgo models`", target)
				}
				m, err := mapping.LoadModelMappings()
				if err != nil {
					return err
				}
				if m[tool] == nil {
					m[tool] = map[string]string{}
				}
				m[tool][source] = models.NormalizeID(target)
				if err := mapping.SaveModelMappings(m); err != nil {
					return err
				}
				fmt.Printf("%s %s -> %s\n", tool, source, models.NormalizeID(target))
				return nil
			},
		},
		&cobra.Command{
			Use: "unset <source-model>", Aliases: []string{"rm", "remove", "delete"}, Short: "Remove one model mapping", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				source := strings.TrimSpace(args[0])
				if source == "" {
					return errors.New("source model cannot be empty")
				}
				m, err := mapping.LoadModelMappings()
				if err != nil {
					return err
				}
				if m[tool] == nil {
					m[tool] = map[string]string{}
				}
				if _, ok := m[tool][source]; !ok {
					return fmt.Errorf("no mapping for %q in %s", source, tool)
				}
				delete(m[tool], source)
				if err := mapping.SaveModelMappings(m); err != nil {
					return err
				}
				fmt.Printf("removed %s mapping for %s\n", tool, source)
				return nil
			},
		},
		&cobra.Command{
			Use: "open", Short: "Open mapping file in $EDITOR",
			RunE: func(cmd *cobra.Command, args []string) error {
				m, err := mapping.LoadModelMappings()
				if err != nil {
					return err
				}
				if _, err := os.Stat(config.ModelMappingFile()); err != nil {
					if err := mapping.SaveModelMappings(m); err != nil {
						return err
					}
				}
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "vi"
				}
				c := exec.Command(editor, config.ModelMappingFile())
				c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
				return c.Run()
			},
		},
	)
	return cmd
}
