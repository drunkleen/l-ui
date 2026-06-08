package arch

import (
	"runtime"
	"strings"

	"github.com/drunkleen/l-ui/internal/util/common"
)

var SupportedTargetArchs = map[string]struct{}{
	"amd64": {},
	"arm64": {},
	"386":   {},
	"armv7": {},
	"armv6": {},
	"armv5": {},
	"s390x": {},
}

func SSHBootstrapArch(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	case "armv7l", "armv7":
		return "armv7", nil
	case "armv6l", "armv6":
		return "armv6", nil
	case "armv5tel", "armv5":
		return "armv5", nil
	case "i386", "i686", "386":
		return "386", nil
	case "s390x":
		return "s390x", nil
	default:
		return "", common.NewError("unsupported remote architecture: " + raw)
	}
}

func CaddyAssetArch(arch string) (string, error) {
	switch strings.TrimSpace(arch) {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	case "armv7":
		return "armv7", nil
	case "armv6":
		return "armv6", nil
	case "386":
		return "386", nil
	case "s390x":
		return "s390x", nil
	default:
		return "", common.NewError("unsupported caddy architecture: " + arch)
	}
}

func NormalizeBundleArch(targetArch string) string {
	targetArch = strings.TrimSpace(targetArch)
	if targetArch == "" {
		return runtime.GOARCH
	}
	return targetArch
}
