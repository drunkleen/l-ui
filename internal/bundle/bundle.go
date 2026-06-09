package bundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/util/arch"
	"github.com/drunkleen/l-ui/internal/util/retry"
	"gorm.io/gorm"
)

type NodeBundle struct {
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	Arch     string    `json:"arch"`
	Path     string    `json:"path"`
	SHA256   string    `json:"sha256"`
	Size     int64     `json:"size"`
	BuiltAt  time.Time `json:"builtAt"`
	Manifest string    `json:"manifestPath,omitempty"`
}

func BuildNodeBundle(targetArch string) (*NodeBundle, error) {
	version, err := BundleVersion()
	if err != nil {
		return nil, err
	}
	targetArch = arch.NormalizeBundleArch(targetArch)
	if _, ok := arch.SupportedTargetArchs[targetArch]; !ok {
		return nil, fmt.Errorf("unsupported target architecture: %s", targetArch)
	}
	if cached, err := LoadCachedNodeBundle(version, targetArch); err != nil {
		return nil, err
	} else if cached != nil {
		return cached, nil
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("node bundles are only supported on linux hosts")
	}

	bundleDir := os.Getenv("LUI_NODE_BUNDLE_DIR")
	if bundleDir == "" {
		bundleDir = filepath.Join("artifacts", "node-bundles")
	}
	if !filepath.IsAbs(bundleDir) {
		bundleDir = filepath.Clean(bundleDir)
	}
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		// Fall back to a temp directory if the configured path is not
		// writable (e.g. Docker container, read-only filesystem).
		fallback := filepath.Join(os.TempDir(), "l-ui-node-bundles")
		if err2 := os.MkdirAll(fallback, 0755); err2 != nil {
			return nil, fmt.Errorf("mkdir %s: %w (tried fallback %s: %w)", bundleDir, err, fallback, err2)
		}
		bundleDir = fallback
	}

	baseName := fmt.Sprintf("l-ui-agent-linux-%s-%s", targetArch, version)
	bundlePath := filepath.Join(bundleDir, baseName+".tar.gz")
	manifestPath := filepath.Join(bundleDir, baseName+".json")

	releaseURL := BundleReleaseURL(version, targetArch)
	// Fallback: if the agent-specific tarball doesn't exist (e.g., old release
	// that predates the hub/agent split), try the old combined tarball name.
	fallbackURL := strings.Replace(releaseURL, "l-ui-agent-linux", "l-ui-linux", 1)
	tmpDir, err := os.MkdirTemp("", "lui-node-bundle-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpPath := filepath.Join(tmpDir, filepath.Base(bundlePath))
	if err := DownloadFileToPath(releaseURL, tmpPath); err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			if err2 := DownloadFileToPath(fallbackURL, tmpPath); err2 != nil {
				return nil, fmt.Errorf("download %s (agent) and %s (combined): %w", releaseURL, fallbackURL, err2)
			}
		} else {
			return nil, fmt.Errorf("download %s: %w", releaseURL, err)
		}
	}
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(bundlePath, content, 0644); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	bundle := &NodeBundle{
		Name:    baseName,
		Version: version,
		Arch:    targetArch,
		Path:    bundlePath,
		SHA256:  hex.EncodeToString(sum[:]),
		Size:    int64(len(content)),
		BuiltAt: time.Now().UTC(),
	}
	manifest, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		return nil, err
	}
	bundle.Manifest = manifestPath
	if db := database.GetDB(); db != nil {
		if err := db.Create(&model.NodeBundle{
			Name:     bundle.Name,
			Version:  bundle.Version,
			Arch:     bundle.Arch,
			SHA256:   bundle.SHA256,
			Path:     bundle.Path,
			Manifest: bundle.Manifest,
			Size:     bundle.Size,
			BuiltAt:  bundle.BuiltAt.UnixMilli(),
		}).Error; err != nil && !strings.Contains(err.Error(), "database is closed") {
			return nil, err
		}
	}
	return bundle, nil
}

func BundleVersion() (string, error) {
	version := strings.TrimSpace(config.GetVersion())
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return "", fmt.Errorf("panel version is unavailable; cannot resolve release bundle")
	}
	return version, nil
}

func BundleReleaseURL(version, arch string) string {
	releaseBase := strings.TrimRight(os.Getenv("LUI_NODE_BUNDLE_RELEASE_BASE"), "/")
	if releaseBase == "" {
		releaseBase = "https://github.com/drunkleen/l-ui/releases/download"
	}
	// Agent-specific tarball (v0.0.1+ split). Falls back to the old combined
	// tarball for releases that predate the split (see DownloadFileToPath).
	return fmt.Sprintf("%s/v%s/l-ui-agent-linux-%s.tar.gz", releaseBase, version, arch)
}

func DownloadFileToPath(url, dst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return retry.Do(ctx, retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Second,
		JitterFactor:   0.2,
	}, func(ctx context.Context) error {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
		}
		file, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(file, resp.Body); err != nil {
			return err
		}
		return file.Sync()
	})
}

func LoadCachedNodeBundle(version, arch string) (*NodeBundle, error) {
	db := database.GetDB()
	if db == nil {
		return nil, nil
	}
	var bundles []model.NodeBundle
	if err := db.Where("version = ? AND arch = ?", version, arch).Order("built_at desc").Find(&bundles).Error; err != nil {
		if strings.Contains(err.Error(), "database is closed") {
			return nil, nil
		}
		return nil, err
	}
	for _, bundle := range bundles {
		if bundle.Path == "" {
			continue
		}
		if _, err := os.Stat(bundle.Path); err != nil {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		}
		if ok, err := CachedBundleMatchesSHA256(bundle.Path, bundle.SHA256); err != nil {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		} else if !ok {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		}
		if ok, err := CachedBundleHasExecutable(bundle.Path); err != nil {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		} else if !ok {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		}
		if ok, err := CachedBundleHasServiceFile(bundle.Path); err != nil {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		} else if !ok {
			_ = DiscardCachedNodeBundle(db, &bundle)
			continue
		}
		manifest := bundle.Manifest
		if manifest != "" {
			if _, err := os.Stat(manifest); err != nil {
				manifest = ""
			}
		}
		return &NodeBundle{
			Name:     bundle.Name,
			Version:  bundle.Version,
			Arch:     bundle.Arch,
			Path:     bundle.Path,
			SHA256:   bundle.SHA256,
			Size:     bundle.Size,
			BuiltAt:  time.UnixMilli(bundle.BuiltAt).UTC(),
			Manifest: manifest,
		}, nil
	}
	return nil, nil
}

func CachedBundleHasServiceFile(bundlePath string) (bool, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return false, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !strings.HasPrefix(hdr.Name, "l-ui-agent/") {
			continue
		}
		switch filepath.Base(hdr.Name) {
		case "l-ui-agent.service", "l-ui-agent.service.debian", "l-ui-agent.service.arch", "l-ui-agent.service.rhel":
			return true, nil
		}
	}
}

func CachedBundleHasExecutable(bundlePath string) (bool, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return false, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if hdr.Name == "l-ui-agent/l-ui-agent" {
			return true, nil
		}
	}
}

func CachedBundleMatchesSHA256(bundlePath, expected string) (bool, error) {
	if strings.TrimSpace(expected) == "" {
		return false, nil
	}
	file, err := os.Open(bundlePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}
	return hex.EncodeToString(hash.Sum(nil)) == strings.ToLower(strings.TrimSpace(expected)), nil
}

func DiscardCachedNodeBundle(db *gorm.DB, bundle *model.NodeBundle) error {
	if bundle == nil {
		return nil
	}
	if bundle.Path != "" {
		_ = os.Remove(bundle.Path)
	}
	if bundle.Manifest != "" {
		_ = os.Remove(bundle.Manifest)
	}
	return db.Delete(bundle).Error
}

func SelectRollbackBundle(current *model.NodeBundle, bundles []*model.NodeBundle) *model.NodeBundle {
	var selected *model.NodeBundle
	for _, candidate := range bundles {
		if candidate == nil || current == nil {
			continue
		}
		if candidate.Arch != current.Arch || candidate.SHA256 == current.SHA256 {
			continue
		}
		if selected == nil || candidate.BuiltAt > selected.BuiltAt {
			selected = candidate
		}
	}
	return selected
}

func LatestNodeBundle() (*model.NodeBundle, error) {
	var bundle model.NodeBundle
	if err := database.GetDB().Order("built_at desc").First(&bundle).Error; err != nil {
		return nil, err
	}
	return &bundle, nil
}
