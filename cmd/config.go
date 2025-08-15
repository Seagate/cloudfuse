package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Launch the interactive configuration TUI",
	Long:  "Starts an interactive terminal-based UI to generate or edit your Cloudfuse configuration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runTUI(); err != nil {
			return fmt.Errorf("failed to run TUI: %v", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}