package txt

import "testing"

func TestOpenBytesChapters(t *testing.T) {
	book, err := OpenBytes("notes.txt", []byte("Глава 1\nПервый абзац.\n\nГлава 2\nВторой абзац про сад.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if book.Format != "txt" {
		t.Fatalf("format %q", book.Format)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters %d", len(book.Chapters))
	}
	html, err := book.ChapterHTML(1)
	if err != nil {
		t.Fatal(err)
	}
	if html == "" {
		t.Fatal("empty html")
	}
	hits, err := book.Search("сад", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("search missed txt")
	}
}
