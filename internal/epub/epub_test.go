package epub

import (
	"strings"
	"testing"
)

func TestSampleBook(t *testing.T) {
	data, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	book, err := OpenBytes("demo.epub", data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })

	if book.Title != "Записки на полях" {
		t.Fatalf("title: %q", book.Title)
	}
	if book.Author != "boo" {
		t.Fatalf("author: %q", book.Author)
	}
	if book.Identifier != SampleIdentifier {
		t.Fatalf("id: %q", book.Identifier)
	}
	if len(book.Chapters) != 3 {
		t.Fatalf("chapters: %d", len(book.Chapters))
	}
	if book.Chapters[0].Title != "Начало" {
		t.Fatalf("chapter title: %q", book.Chapters[0].Title)
	}
	if len(book.TOC) != 3 {
		t.Fatalf("toc: %#v", book.TOC)
	}
	if len(book.TOC[1].Children) != 2 {
		t.Fatalf("nested toc: %#v", book.TOC[1])
	}
	if book.CoverHref != "OEBPS/cover.png" {
		t.Fatalf("cover: %q", book.CoverHref)
	}

	html, err := book.ChapterHTML(1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "alert") || strings.Contains(html, "<script") {
		t.Fatalf("script leaked: %s", html)
	}
	if !strings.Contains(html, `/res?p=OEBPS%2Fcover.png`) && !strings.Contains(html, `/res?p=OEBPS/cover.png`) {
		t.Fatalf("image src not rewritten: %s", html)
	}
	if !strings.Contains(html, `data-href="OEBPS/ch3.xhtml"`) {
		t.Fatalf("internal link not rewritten: %s", html)
	}
	if !strings.Contains(html, `class="table-wrap"`) || !strings.Contains(html, "data-table") {
		t.Fatalf("demo table not formatted: %s", html)
	}
	if !strings.Contains(html, "Клавиши и жесты") {
		t.Fatalf("table caption missing: %s", html)
	}

	raw, mime, err := book.Resource("OEBPS/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" {
		t.Fatalf("mime: %q", mime)
	}
	if len(raw) < 50 {
		t.Fatalf("tiny cover: %d", len(raw))
	}
	if book.ChapterIndexByHref("OEBPS/ch3.xhtml") != 2 {
		t.Fatal("chapter index")
	}

	sub, ok := LookupTOC(book.TOC, "1.0")
	if !ok || sub.Title != "Клавиши" || sub.Fragment != "keys" || sub.ChapterIndex != 1 {
		t.Fatalf("subchapter %#v %v", sub, ok)
	}
	keys := TOCKeysForChapter(book.TOC, 1)
	if len(keys) != 3 || keys[0] != "1" || keys[1] != "1.0" || keys[2] != "1.1" {
		t.Fatalf("chapter keys %#v", keys)
	}
	if ValidTOCKey("") || ValidTOCKey("1.") || ValidTOCKey("a.1") || !ValidTOCKey("1.0") {
		t.Fatal("toc key")
	}
}

func TestResolvePath(t *testing.T) {
	got := resolvePath("OEBPS/Text/ch1.xhtml", "../Images/cover.jpg")
	if got != "OEBPS/Images/cover.jpg" {
		t.Fatalf("got %q", got)
	}
}
