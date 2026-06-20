package testutil_test
import (
	"recipe-server/internal/testutil"
	"os"
	"path/filepath"
	"testing"

	"recipe-server/config"
)

func TestFindConfigYAMLFromModuleRoot(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for filepath.Base(root) != "testutil" {
		parent := filepath.Dir(root)
		if parent == root {
			t.Skip("未找到 testutil 目录")
		}
		root = parent
	}
	moduleRoot := filepath.Dir(root)

	example := filepath.Join(moduleRoot, "config.yaml.example")
	if _, err := os.Stat(example); err != nil {
		t.Skip("无 config.yaml.example")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	data, _ := os.ReadFile(example)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(tmp, "internal", "service")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(root) })

	found, ok := testutil.FindConfigYAML()
	if !ok {
		t.Fatal("应向上找到 config.yaml")
	}
	if found != cfgPath {
		t.Fatalf("path: got %q want %q", found, cfgPath)
	}
}

func TestEnsureAppConfigUsesLoadedYAML(t *testing.T) {
	root, _ := os.Getwd()
	for filepath.Base(root) != "testutil" {
		root = filepath.Dir(root)
	}
	moduleRoot := filepath.Dir(root)

	example := filepath.Join(moduleRoot, "config.yaml.example")
	if _, err := os.Stat(example); err != nil {
		t.Skip("无 config.yaml.example")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := "jwt:\n  secret: from-yaml-secret\n  expire_hours: 48\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "pkg", "jwt")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(root) })

	old := config.AppConfig
	config.AppConfig = nil
	t.Cleanup(func() { config.AppConfig = old })

	testutil.EnsureAppConfig()
	if config.AppConfig.JWT.Secret != "from-yaml-secret" {
		t.Fatalf("应使用 yaml 中的 secret, got %q", config.AppConfig.JWT.Secret)
	}
	if config.AppConfig.JWT.ExpireHours != 48 {
		t.Fatalf("应使用 yaml 中的 expire_hours, got %d", config.AppConfig.JWT.ExpireHours)
	}
}

func TestEnsureAppConfigFallsBackWhenNoYAML(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	old := config.AppConfig
	config.AppConfig = nil
	t.Cleanup(func() { config.AppConfig = old })

	testutil.EnsureAppConfig()
	if config.AppConfig.JWT.Secret != "test-secret" {
		t.Fatalf("无 yaml 时应补默认 secret, got %q", config.AppConfig.JWT.Secret)
	}
}
