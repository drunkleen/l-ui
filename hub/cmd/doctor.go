package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate configuration and environment",
	Run: func(cmd *cobra.Command, args []string) {
		if !runDoctor(doctorJSON) {
			os.Exit(1)
		}
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output results in JSON format")

	rootCmd.AddCommand(doctorCmd)
}
