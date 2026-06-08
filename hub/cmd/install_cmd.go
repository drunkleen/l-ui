package cmd

import (
	"github.com/drunkleen/l-ui/hub/cmd/install"
)

func init() {
	rootCmd.AddCommand(install.Cmd())
}
