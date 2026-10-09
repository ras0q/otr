package npm_test

import (
	"context"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/npm"
	"github.com/ras0q/otr/outbound/source"
	"github.com/ras0q/otr/outbound/storage/local"
)

type stubSource struct {
	release source.Release
}

func (s stubSource) GetLatestRelease(context.Context, source.Identity) (source.Release, error) {
	return s.release, nil
}

func (s stubSource) DownloadReleaseAsset(context.Context, url.URL) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func TestE2EInstall(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	storageRoot := t.TempDir()
	blobStorage, err := local.New(storageRoot)
	if err != nil {
		t.Fatalf("local storage: %v", err)
	}

	const version = "1.0.0"
	service := app.NewService(stubSource{release: source.Release{
		Version:       version,
		RepositoryURL: "https://github.com/example/tool",
		Assets:        nil,
	}})

	srv := httptest.NewTestServer(t, npm.NewRegistry(service, blobStorage, npm.Config{
		Scope: "@otr",
	}))
	srv.Start()

	fixtureDir := "./e2e/"
	nodeModulesDir := filepath.Join(fixtureDir, "node_modules")
	os.RemoveAll(nodeModulesDir)

	cmd := exec.Command("npm", "install", "--no-fund", "--no-audit")
	cmd.Dir = fixtureDir
	cmd.Env = append(os.Environ(), "OTR_E2E_REGISTRY="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("npm install: %v\n%s", err, out)
	}

	installed := filepath.Join(nodeModulesDir, "@otr", "example--tool", "package.json")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected installed package at %s: %v\nnpm output:\n%s", installed, err, out)
	}
}
