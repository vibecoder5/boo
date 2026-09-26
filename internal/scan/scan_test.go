package scan

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"boo/internal/epub"
)

func TestFolderSkipsTextAndHidden(t *testing.T) {
	root := t.TempDir()
	epubData, err := epub.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "story.epub"), epubData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "summer.fb2"), []byte(fb2Sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFB2Zip(filepath.Join(root, "packed.fb2.zip"), fb2Sample); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("Глава\n\nТекст.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Заметка\n\nТекст.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.epub"), []byte("not an epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTextZip(filepath.Join(root, "extra.zip")); err != nil {
		t.Fatal(err)
	}
	hidden := filepath.Join(root, ".hidden")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "secret.epub"), epubData, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Folder(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unreadable != 1 {
		t.Fatalf("unreadable %d", got.Unreadable)
	}
	if len(got.Books) != 3 {
		t.Fatalf("books %d %#v", len(got.Books), names(got.Books))
	}
	formats := map[string]string{}
	for _, b := range got.Books {
		formats[b.Name] = b.Format
		if b.Format != "epub" && b.Format != "fb2" {
			t.Fatalf("format %s %s", b.Name, b.Format)
		}
	}
	if formats["story.epub"] != "epub" || formats["summer.fb2"] != "fb2" || formats["packed.fb2.zip"] != "fb2" {
		t.Fatalf("%v", formats)
	}
	for _, b := range got.Books {
		if b.Name == "notes.txt" || b.Name == "notes.md" || b.Name == "secret.epub" || b.Name == "extra.zip" {
			t.Fatalf("unexpected %s", b.Name)
		}
	}
}

func TestOpenRejectsText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("Текст.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("txt opened")
	}
}

func names(books []Hit) []string {
	out := make([]string, len(books))
	for i, b := range books {
		out[i] = b.Name
	}
	return out
}

func writeFB2Zip(path, fb2 string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("book.fb2")
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(fb2)); err != nil {
		return err
	}
	return zw.Close()
}

func writeTextZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("notes.txt")
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("текст")); err != nil {
		return err
	}
	return zw.Close()
}

const fb2Sample = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
  <description>
    <title-info>
      <book-title>Летний сад</book-title>
      <author><first-name>Анна</first-name><last-name>Речная</last-name></author>
      <lang>ru</lang>
    </title-info>
  </description>
  <body>
    <section>
      <title><p>Глава</p></title>
      <p>Текст.</p>
    </section>
  </body>
</FictionBook>
`
