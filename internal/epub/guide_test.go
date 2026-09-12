package epub

import (
	"strings"
	"testing"

	"boo/docs"
)

func TestGuideBook(t *testing.T) {
	book, err := Guide(docs.UserGuide)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	if book.Identifier != GuideIdentifier {
		t.Fatalf("id %q", book.Identifier)
	}
	if book.Key != "id:"+GuideIdentifier {
		t.Fatalf("key %q", book.Key)
	}
	if book.Format != "guide" {
		t.Fatalf("format %q", book.Format)
	}
	if book.Title == "" || book.Author != "boo" {
		t.Fatalf("meta %q %q", book.Title, book.Author)
	}
	if len(book.Chapters) < 8 {
		t.Fatalf("chapters %d", len(book.Chapters))
	}
	if book.CoverHref == "" {
		t.Fatal("cover")
	}

	var foundTable, foundFence, foundSub bool
	for i := range book.Chapters {
		html, err := book.ChapterHTML(i)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(html, "alert(") || strings.Contains(html, "<script") {
			t.Fatalf("unsafe html: %s", html)
		}
		if strings.Contains(html, "Клавиши") && strings.Contains(html, "data-table") {
			foundTable = true
		}
		if strings.Contains(html, "<pre>") && strings.Contains(html, "boo.exe") {
			foundFence = true
		}
		if strings.Contains(html, `id="меню-слева"`) {
			foundSub = true
		}
	}
	if !foundTable {
		t.Fatal("keyboard table not rendered")
	}
	if !foundFence {
		t.Fatal("code fence not rendered")
	}
	if !foundSub {
		t.Fatal("subsection id missing")
	}

	var hasChild bool
	for _, item := range book.TOC {
		if len(item.Children) > 0 {
			hasChild = true
			break
		}
	}
	if !hasChild {
		t.Fatal("toc children missing")
	}

	hits, err := book.Search("полка", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("search missed guide")
	}
}
