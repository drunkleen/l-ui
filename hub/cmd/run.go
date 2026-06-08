package cmd

import "github.com/spf13/cobra"

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the hub web panel",
	Run: func(cmd *cobra.Command, args []string) {
		runWebServer()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
