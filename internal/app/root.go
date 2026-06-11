package app

import (
	"github.com/spf13/cobra"
	"ocgo/internal/config"
)

func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{Use: config.AppName, Short: "Run Claude Code with OpenCode Go", Version: version}
	root.AddCommand(SetupCmd(), ListCmd(), MappingCmd(), LaunchCmd(), OpencodeCmd(), ServeCmd(), StopCmd(), StatusCmd(), DaemonCmd(), CodexCmd(), DoctorCmd(), VersionCmd(), ConfigCmd())
	return root
}
