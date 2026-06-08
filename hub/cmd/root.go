package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/spf13/cobra"
)

var showVersion bool

var rootCmd = &cobra.Command{
	Use:   config.GetName(),
	Short: "L-UI hub server",
	Long:  `L-UI is a hub for managing remote VPS nodes and their Xray instances.`,
	Run: func(cmd *cobra.Command, args []string) {
		runMenu()
	},
}

func Execute() {
	loadServiceEnvFile()
	os.Args = normalizeArgs(os.Args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "show version")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(config.GetVersion())
			os.Exit(0)
		}
		return nil
	}
}

var singleDashLongFlags = []string{
	"port", "username", "password", "webBasePath", "listenIP",
	"resetTwoFactor", "getListen", "getCert", "getApiToken",
	"webCert", "webCertKey", "tgbottoken", "tgbotRuntime", "tgbotchatid",
	"enabletgbot", "reset", "show", "dsn", "src", "json",
}

func normalizeArgs(args []string) []string {
	flagSet := make(map[string]bool)
	for _, f := range singleDashLongFlags {
		flagSet[f] = true
	}

	result := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			rest := arg[1:]
			name := rest
			if idx := strings.Index(rest, "="); idx >= 0 {
				name = rest[:idx]
			}
			if flagSet[name] {
				arg = "-" + arg
			}
		}
		result = append(result, arg)
	}
	return result
}
