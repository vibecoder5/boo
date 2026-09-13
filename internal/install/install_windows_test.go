//go:build windows

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func testProfile(t *testing.T) Profile {
	t.Helper()
	return Profile{
		ID:      "boo-test-installer",
		Name:    "boo-test-installer",
		Version: "0.0.0-test",
		ExeName: "boo.exe",
	}
}

func TestInstallUninstall(t *testing.T) {
	p := testProfile(t)
	_ = Uninstall(p)

	root := t.TempDir()
	dir := filepath.Join(root, "app")
	start := filepath.Join(root, "start")
	desk := filepath.Join(root, "desk")
	dataProbe := filepath.Join(root, "should-keep")
	if err := os.WriteFile(dataProbe, []byte("library"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Install(Options{
		Dir:          dir,
		Bundle:       Bundle{App: []byte("fake-boo"), Uninstaller: []byte("fake-uninst")},
		StartMenu:    true,
		Desktop:      true,
		Associations: true,
		Profile:      p,
		StartMenuDir: start,
		DesktopDir:   desk,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Exe == "" {
		t.Fatal("empty exe")
	}
	if _, err := os.Stat(res.Exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Uninstaller); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(start, p.Name+".lnk")); err != nil {
		t.Fatalf("start menu: %v", err)
	}
	if _, err := os.Stat(filepath.Join(desk, p.Name+".lnk")); err != nil {
		t.Fatalf("desktop: %v", err)
	}

	got, ok := Installed(p)
	if !ok || got != dir {
		t.Fatalf("Installed: dir=%q ok=%v", got, ok)
	}
	ukey, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath(p.ID), registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	cmd, _, err := ukey.GetStringValue("UninstallString")
	ukey.Close()
	if err != nil || !strings.Contains(cmd, "-id "+p.ID) {
		t.Fatalf("UninstallString %q %v", cmd, err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, fileClasses+`\.epub`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := key.GetStringValue("")
	key.Close()
	if err != nil || prog != p.progID(".epub") {
		t.Fatalf("epub progid: %q %v", prog, err)
	}

	if err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := Installed(p); ok {
		t.Fatal("still installed")
	}
	if _, err := os.Stat(res.Exe); !os.IsNotExist(err) {
		t.Fatalf("exe left: %v", err)
	}
	if _, err := os.Stat(dataProbe); err != nil {
		t.Fatal("деинсталлятор не должен трогать посторонние файлы")
	}

	key, err = registry.OpenKey(registry.CURRENT_USER, fileClasses+`\`+p.progID(".epub"), registry.QUERY_VALUE)
	if err == nil {
		key.Close()
		t.Fatal("progid остался")
	}
}

func TestUninstallMissing(t *testing.T) {
	err := Uninstall(Profile{ID: "boo-test-installer-missing"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallRequiresApp(t *testing.T) {
	_, err := Install(Options{Dir: t.TempDir(), Profile: testProfile(t)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "boo" {
		t.Fatalf("dir=%q", dir)
	}
}
