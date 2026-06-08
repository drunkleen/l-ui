package cmd

import "github.com/spf13/cobra"

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run the legacy database migration",
	Run: func(cmd *cobra.Command, args []string) {
		migrateDb()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
