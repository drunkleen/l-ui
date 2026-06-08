package cmd

import (
	"github.com/spf13/cobra"
)

var (
	settingPort           int
	settingUsername       string
	settingPassword       string
	settingWebBasePath    string
	settingListenIP       string
	settingResetTwoFactor bool
	settingGetListen      bool
	settingGetCert        bool
	settingGetApiToken    bool
	settingWebCert        string
	settingWebCertKey     string
	settingTgbotToken     string
	settingTgbotRuntime   string
	settingTgbotChatid    string
	settingEnableTgbot    bool
	settingReset          bool
	settingShow           bool
	settingValidate       bool
)

func addSharedSettingFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&settingReset, "reset", false, "Reset all settings")
	cmd.Flags().BoolVar(&settingShow, "show", false, "Display current settings")
	cmd.Flags().BoolVar(&settingValidate, "validate", false, "Run validations without applying changes")
	cmd.Flags().IntVar(&settingPort, "port", 0, "Set panel port number")
	cmd.Flags().StringVar(&settingUsername, "username", "", "Set login username")
	cmd.Flags().StringVar(&settingPassword, "password", "", "Set login password")
	cmd.Flags().StringVar(&settingWebBasePath, "webBasePath", "", "Set base path for Panel")
	cmd.Flags().StringVar(&settingListenIP, "listenIP", "", "set panel listenIP IP")
	cmd.Flags().BoolVar(&settingResetTwoFactor, "resetTwoFactor", false, "Reset two-factor authentication settings")
	cmd.Flags().BoolVar(&settingGetListen, "getListen", false, "Display current panel listenIP IP")
	cmd.Flags().BoolVar(&settingGetCert, "getCert", false, "Display current certificate settings")
	cmd.Flags().BoolVar(&settingGetApiToken, "getApiToken", false, "Display current API token")
	cmd.Flags().StringVar(&settingWebCert, "webCert", "", "Set path to public key file for panel")
	cmd.Flags().StringVar(&settingWebCertKey, "webCertKey", "", "Set path to private key file for panel")
	cmd.Flags().StringVar(&settingTgbotToken, "tgbottoken", "", "Set token for Telegram bot")
	cmd.Flags().StringVar(&settingTgbotRuntime, "tgbotRuntime", "", "Set cron time for Telegram bot notifications")
	cmd.Flags().StringVar(&settingTgbotChatid, "tgbotchatid", "", "Set chat ID for Telegram bot notifications")
	cmd.Flags().BoolVar(&settingEnableTgbot, "enabletgbot", false, "Enable notifications via Telegram bot")
}

var settingCmd = &cobra.Command{
	Use:     "setting",
	Aliases: []string{"settings"},
	Short:   "View or update panel settings",
	Run: func(cmd *cobra.Command, args []string) {
		if settingValidate {
			runSettingValidation()
			return
		}
		if settingReset {
			if err := resetSetting(); err != nil {
				return
			}
		} else {
			if err := updateSetting(settingPort, settingUsername, settingPassword, settingWebBasePath, settingListenIP, settingResetTwoFactor); err != nil {
				return
			}
		}
		if settingShow {
			showSetting(settingShow)
		}
		if settingGetListen {
			GetListenIP(settingGetListen)
		}
		if settingGetCert {
			GetCertificate(settingGetCert)
		}
		if settingGetApiToken {
			GetApiToken(settingGetApiToken)
		}
		if settingTgbotToken != "" || settingTgbotChatid != "" || settingTgbotRuntime != "" {
			updateTgbotSetting(settingTgbotToken, settingTgbotChatid, settingTgbotRuntime)
		}
		if settingEnableTgbot {
			updateTgbotEnableSts(settingEnableTgbot)
		}
	},
}

func init() {
	addSharedSettingFlags(settingCmd)
	rootCmd.AddCommand(settingCmd)
}
