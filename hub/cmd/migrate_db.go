package cmd

import (
	"fmt"
	"os"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/spf13/cobra"
)

var (
	migrateDsn string
	migrateSrc string
)

var migrateDbCmd = &cobra.Command{
	Use:   "migrate-db",
	Short: "Copy SQLite data into PostgreSQL",
	Run: func(cmd *cobra.Command, args []string) {
		src := migrateSrc
		if src == "" {
			src = config.GetDBPath()
		}
		if migrateDsn == "" {
			fmt.Println("--dsn is required: postgres://user:pass@host:port/dbname?sslmode=disable")
			os.Exit(1)
		}
		if err := database.MigrateData(src, migrateDsn); err != nil {
			fmt.Println("migration failed:", err)
			os.Exit(1)
		}
	},
}

func init() {
	migrateDbCmd.Flags().StringVar(&migrateDsn, "dsn", "", "Destination PostgreSQL DSN (postgres://user:pass@host:port/db?sslmode=disable)")
	migrateDbCmd.Flags().StringVar(&migrateSrc, "src", "", "Source SQLite file (defaults to the configured l-ui.db)")

	rootCmd.AddCommand(migrateDbCmd)
}
