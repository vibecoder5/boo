package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDirName(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "boo" {
		t.Fatalf("dir=%q", dir)
	}
	if !strings.Contains(filepath.ToSlash(dir), "Programs/boo") && filepath.Base(filepath.Dir(dir)) != "Programs" {
		t.Logf("каталог не стандартный (допустимо): %s", dir)
	}
}

func TestDataDirNotInstallDir(t *testing.T) {
	data := DataDir()
	inst, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if data != "" && data == inst {
		t.Fatal("данные и программа не должны жить в одном каталоге")
	}
}

func TestProgID(t *testing.T) {
	p := Profile{ID: "boo"}.withDefaults()
	if p.progID(".epub") != "boo.epub" {
		t.Fatal(p.progID(".epub"))
	}
}
