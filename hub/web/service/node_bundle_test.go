package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/bundle"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func TestBuildNodeBundleDownloadsAndCachesRelease(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUI_NODE_BUNDLE_DIR", dir)
	t.Setenv("LUI_LOG_FOLDER", dir)
	t.Setenv("LUI_BIN_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	archive := makeReleaseBundleArchive(t, goruntime.GOARCH)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if _, err := w.Write(archive); err != nil {
			t.Fatalf("write archive: %v", err)
		}
	}))
	defer server.Close()
	t.Setenv("LUI_NODE_BUNDLE_RELEASE_BASE", server.URL)

	bnd, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("buildNodeBundle returned error: %v", err)
	}
	second, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("cached buildNodeBundle returned error: %v", err)
	}
	if second.Path != bnd.Path || second.SHA256 != bnd.SHA256 {
		t.Fatalf("cache mismatch: %#v != %#v", second, bnd)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 release download, got %d", hits)
	}
	if bnd == nil || bnd.Path == "" || bnd.Manifest == "" {
		t.Fatalf("unexpected bundle result: %#v", bnd)
	}
	if _, err := os.Stat(bnd.Path); err != nil {
		t.Fatalf("bundle file missing: %v", err)
	}
	manifestBytes, err := os.ReadFile(bnd.Manifest)
	if err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	var manifest bundle.NodeBundle
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.SHA256 != bnd.SHA256 || manifest.Path != bnd.Path || manifest.Version != bnd.Version {
		t.Fatalf("manifest mismatch: %#v != %#v", manifest, bnd)
	}
	content, err := os.ReadFile(bnd.Path)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	gr, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	foundNode := false
	foundXray := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		switch hdr.Name {
		case "l-ui-agent/l-ui-agent":
			foundNode = true
		case filepath.ToSlash(filepath.Join("l-ui-agent", "bin", "xray-linux-"+goruntime.GOARCH)):
			foundXray = true
		}
	}
	if !foundNode || !foundXray {
		t.Fatalf("bundle contents missing: node=%v xray=%v", foundNode, foundXray)
	}
}

func TestBuildNodeBundleReusesCachedVersionArch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUI_NODE_BUNDLE_DIR", dir)
	t.Setenv("LUI_LOG_FOLDER", dir)
	t.Setenv("LUI_BIN_FOLDER", dir)

	version, err := bundle.BundleVersion()
	if err != nil {
		t.Fatalf("bundleVersion: %v", err)
	}
	bundlePath := filepath.Join(dir, "cached.tar.gz")
	archive := makeReleaseBundleArchive(t, goruntime.GOARCH)
	sum := sha256.Sum256(archive)
	if err := os.WriteFile(bundlePath, archive, 0600); err != nil {
		t.Fatalf("write cached bundle: %v", err)
	}
	manifestPath := filepath.Join(dir, "cached.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":"`+version+`"}`), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	seed := model.NodeBundle{
		Name:     "cached",
		Version:  version,
		Arch:     goruntime.GOARCH,
		SHA256:   hex.EncodeToString(sum[:]),
		Path:     bundlePath,
		Manifest: manifestPath,
		Size:     int64(len(archive)),
		BuiltAt:  123,
	}
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()
	if err := database.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	bnd, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("buildNodeBundle returned error: %v", err)
	}
	if bnd.Path != bundlePath || bnd.SHA256 != seed.SHA256 || bnd.Version != version {
		t.Fatalf("unexpected cached bundle: %#v", bnd)
	}
}

func TestBuildNodeBundleInvalidatesCachedBundleWithWrongHash(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUI_NODE_BUNDLE_DIR", dir)
	t.Setenv("LUI_LOG_FOLDER", dir)
	t.Setenv("LUI_BIN_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	version, err := bundle.BundleVersion()
	if err != nil {
		t.Fatalf("bundleVersion: %v", err)
	}
	bundlePath := filepath.Join(dir, "cached.tar.gz")
	archive := makeReleaseBundleArchive(t, goruntime.GOARCH)
	if err := os.WriteFile(bundlePath, archive, 0600); err != nil {
		t.Fatalf("write cached bundle: %v", err)
	}
	manifestPath := filepath.Join(dir, "cached.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":"`+version+`"}`), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	seed := model.NodeBundle{
		Name:     "cached",
		Version:  version,
		Arch:     goruntime.GOARCH,
		SHA256:   "definitely-wrong",
		Path:     bundlePath,
		Manifest: manifestPath,
		Size:     int64(len(archive)),
		BuiltAt:  123,
	}
	if err := database.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if _, err := w.Write(archive); err != nil {
			t.Fatalf("write archive: %v", err)
		}
	}))
	defer server.Close()
	t.Setenv("LUI_NODE_BUNDLE_RELEASE_BASE", server.URL)

	bnd, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("buildNodeBundle returned error: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected a fresh release download, got %d", hits)
	}
	if bnd.Path == bundlePath {
		t.Fatalf("expected rebuilt bundle to replace invalid cached path %s", bundlePath)
	}
	if _, err := os.Stat(bnd.Path); err != nil {
		t.Fatalf("rebuilt bundle missing: %v", err)
	}
	if _, err := os.Stat(bundlePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale cached bundle to be removed, got err=%v", err)
	}
}

func TestBuildNodeBundleSkipsCachedBundleMissingService(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUI_NODE_BUNDLE_DIR", dir)
	t.Setenv("LUI_LOG_FOLDER", dir)
	t.Setenv("LUI_BIN_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	version, err := bundle.BundleVersion()
	if err != nil {
		t.Fatalf("bundleVersion: %v", err)
	}
	invalidPath := filepath.Join(dir, "invalid.tar.gz")
	if err := os.WriteFile(invalidPath, []byte("broken-bundle"), 0600); err != nil {
		t.Fatalf("write invalid bundle: %v", err)
	}
	manifestPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":"`+version+`"}`), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	seed := model.NodeBundle{
		Name:     "invalid",
		Version:  version,
		Arch:     goruntime.GOARCH,
		SHA256:   "seed-sha",
		Path:     invalidPath,
		Manifest: manifestPath,
		Size:     int64(len("broken-bundle")),
		BuiltAt:  123,
	}
	if err := database.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	archive := makeReleaseBundleArchive(t, goruntime.GOARCH)
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if _, err := w.Write(archive); err != nil {
			t.Fatalf("write archive: %v", err)
		}
	}))
	defer server.Close()
	t.Setenv("LUI_NODE_BUNDLE_RELEASE_BASE", server.URL)

	bnd, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("buildNodeBundle returned error: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected a fresh release download, got %d", hits)
	}
	if bnd.Path == invalidPath {
		t.Fatalf("expected rebuilt bundle to replace invalid path %s", invalidPath)
	}
	if _, err := os.Stat(bnd.Path); err != nil {
		t.Fatalf("rebuilt bundle missing: %v", err)
	}
}

func TestSelectRollbackBundle(t *testing.T) {
	current := &model.NodeBundle{Arch: "amd64", SHA256: "current", BuiltAt: 200}
	candidates := []*model.NodeBundle{
		{Arch: "arm64", SHA256: "wrong-arch", BuiltAt: 300},
		{Arch: "amd64", SHA256: "older", BuiltAt: 100},
		{Arch: "amd64", SHA256: "rollback", BuiltAt: 150},
		{Arch: "amd64", SHA256: "current", BuiltAt: 250},
	}
	got := bundle.SelectRollbackBundle(current, candidates)
	if got == nil || got.SHA256 != "rollback" {
		t.Fatalf("selectRollbackBundle = %#v, want rollback", got)
	}
	if bundle.SelectRollbackBundle(current, []*model.NodeBundle{{Arch: "arm64", SHA256: "a", BuiltAt: time.Now().UnixMilli()}}) != nil {
		t.Fatal("expected nil rollback selection for incompatible bundles")
	}
}

func makeReleaseBundleArchive(t *testing.T, arch string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	write := func(name string, data []byte, mode int64) {
		t.Helper()
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: time.Now(), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write data %s: %v", name, err)
		}
	}
	serviceDebian := mustReadTestFile(t, filepath.Join("l-ui-agent.service.debian"))
	serviceArch := mustReadTestFile(t, filepath.Join("l-ui-agent.service.arch"))
	serviceRhel := mustReadTestFile(t, filepath.Join("l-ui-agent.service.rhel"))
	write("l-ui-agent/l-ui-agent", []byte("node-binary"), 0755)
	write("l-ui-agent/l-ui-agent.service", serviceDebian, 0644)
	write("l-ui-agent/l-ui-agent.service.debian", serviceDebian, 0644)
	write("l-ui-agent/l-ui-agent.service.arch", serviceArch, 0644)
	write("l-ui-agent/l-ui-agent.service.rhel", serviceRhel, 0644)
	write(filepath.ToSlash(filepath.Join("l-ui-agent", "bin", "xray-linux-"+arch)), []byte("xray-binary"), 0755)
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
