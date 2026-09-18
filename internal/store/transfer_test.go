package store

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportImport(t *testing.T) {
	dirA := t.TempDir()
	t.Setenv("APPDATA", dirA)
	t.Setenv("XDG_CONFIG_HOME", dirA)
	src, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	bookPath, err := src.SaveBookFile("id:1", "story.fb2", []byte("<FictionBook>книга</FictionBook>"))
	if err != nil {
		t.Fatal(err)
	}
	cover, err := src.SaveCover("id:1", "image/png", []byte("png-cover"))
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Remember(Entry{
		Key: "id:1", Title: "Книга", Author: "А", Path: bookPath, Format: "fb2",
		Cover: cover, ChapterN: 8, Description: "о книге", Journal: "заметка",
	}); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveProgress("id:1", Progress{Title: "Книга", Author: "А", ChapterIndex: 3, ScrollRatio: 0.4}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.AddBookmark(Bookmark{BookKey: "id:1", Title: "Место", ChapterIndex: 2, ScrollRatio: 0.5}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.AddNote(Note{BookKey: "id:1", ChapterIndex: 1, Start: 0, End: 4, Color: "blue", Text: "фраг", Body: "мысль"}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.CreateList("Отпуск", "id:1"); err != nil {
		t.Fatal(err)
	}
	if _, err := src.AddTodo(Todo{BookKey: "id:1", Text: "дочитать", DueAt: time.Date(2026, 9, 20, 18, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}
	dict, err := src.AddDictionary(Dictionary{Name: "EN-RU", FromLang: "en", ToLang: "ru", FileName: "en-ru.txt", Ext: ".txt", WordCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.SaveDictionaryFile(dict, []byte("hello\tпривет\n")); err != nil {
		t.Fatal(err)
	}
	if err := src.SetBookDictionary("id:1", dict.ID); err != nil {
		t.Fatal(err)
	}
	if err := src.SetUI(UI{Theme: "sepia", FontSize: 22, LineHeight: 1.8, MaxWidth: 42, SidebarWidth: 300, SidebarOpen: true, NotesWidth: 280, NotesOpen: true, HistoryWidth: 260}); err != nil {
		t.Fatal(err)
	}
	bgName, err := src.SaveWelcomeBackground(tinyPNG)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := src.Export(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty archive")
	}

	dirB := t.TempDir()
	t.Setenv("APPDATA", dirB)
	t.Setenv("XDG_CONFIG_HOME", dirB)
	dst, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Remember(Entry{Key: "id:old", Title: "Старая"}); err != nil {
		t.Fatal(err)
	}
	if err := dst.Import(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}

	items := dst.Library()
	if len(items) != 1 || items[0].Title != "Книга" || items[0].Description != "о книге" || items[0].Journal != "заметка" {
		t.Fatalf("library %#v", items)
	}
	if items[0].DictionaryID != dict.ID {
		t.Fatalf("dictionary %#v", items[0])
	}
	if items[0].Path == bookPath || !strings.Contains(items[0].Path, dirB) {
		t.Fatalf("path not remapped %#v", items[0].Path)
	}
	data, err := os.ReadFile(items[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("книга")) {
		t.Fatalf("book file %s", data)
	}
	coverPath, err := dst.CoverFile(items[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(coverPath); err != nil || string(got) != "png-cover" {
		t.Fatalf("cover %s %v", got, err)
	}
	p, ok := dst.Progress("id:1")
	if !ok || p.ChapterIndex != 3 {
		t.Fatalf("progress %#v %v", p, ok)
	}
	if len(dst.Bookmarks("id:1")) != 1 || len(dst.Notes("id:1")) != 1 {
		t.Fatal("marks")
	}
	if lists := dst.Lists(); len(lists) != 1 || lists[0].Name != "Отпуск" || lists[0].BookCount != 1 {
		t.Fatalf("lists %#v", lists)
	}
	if todos := dst.Todos("id:1"); len(todos) != 1 || todos[0].Text != "дочитать" {
		t.Fatalf("todos %#v", todos)
	}
	dicts := dst.Dictionaries()
	if len(dicts) != 1 || dicts[0].Name != "EN-RU" {
		t.Fatalf("dicts %#v", dicts)
	}
	dictPath, err := dst.DictionaryFile(dicts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(dictPath); err != nil || !bytes.Contains(got, []byte("привет")) {
		t.Fatalf("dict file %s %v", got, err)
	}
	ui := dst.UI()
	if ui.Theme != "sepia" || ui.FontSize != 22 || ui.BookFontSize != 22 || ui.BookFont != "serif" || ui.UIFont != "system" || ui.UIFontSize != 16 {
		t.Fatalf("ui %#v", ui)
	}
	if ui.WelcomeBackground != bgName {
		t.Fatalf("background name %#v", ui.WelcomeBackground)
	}
	bgPath, err := dst.WelcomeBackgroundFile()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(bgPath); err != nil || !bytes.Equal(got, tinyPNG) {
		t.Fatalf("background file %s %v", got, err)
	}
	if _, ok := dst.Entry("id:old"); ok {
		t.Fatal("old book should be replaced")
	}
}

func TestImportRejectsBadArchive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:keep", Title: "Оставить"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Import(bytes.NewReader([]byte("not a zip"))); err == nil {
		t.Fatal("expected error")
	}
	if _, ok := st.Entry("id:keep"); !ok {
		t.Fatal("failed import wiped library")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("no state"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := st.Import(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected missing state")
	}
	if _, ok := st.Entry("id:keep"); !ok {
		t.Fatal("bad zip wiped library")
	}
}

func TestImportRejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("state.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`{"ui":{"theme":"dark","fontSize":20,"lineHeight":1.7,"maxWidth":38,"sidebarWidth":280,"notesWidth":300,"historyWidth":280}}`))
	slip, err := zw.Create("library/../../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = slip.Write([]byte("no"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := st.Import(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected zip slip error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatal("escaped file")
	}
}

func TestExportImportGuide(t *testing.T) {
	dirA := t.TempDir()
	t.Setenv("APPDATA", dirA)
	t.Setenv("XDG_CONFIG_HOME", dirA)
	src, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Remember(Entry{
		Key: "id:urn:uuid:boo-guide", Title: "Руководство пользователя", Author: "boo", Format: "guide", ChapterN: 12,
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := src.Export(&buf); err != nil {
		t.Fatal(err)
	}

	dirB := t.TempDir()
	t.Setenv("APPDATA", dirB)
	t.Setenv("XDG_CONFIG_HOME", dirB)
	dst, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Import(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	items := dst.Library()
	if len(items) != 1 || items[0].Format != "guide" || items[0].Path != "" {
		t.Fatalf("library %#v", items)
	}
}
