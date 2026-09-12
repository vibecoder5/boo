package epub

import (
	"strings"
	"testing"
)

func TestSearchDemo(t *testing.T) {
	data, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	book, err := OpenBytes("demo.epub", data)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := book.Search("оглавление", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if hits[0].ChapterIndex < 0 {
		t.Fatalf("chapter %d", hits[0].ChapterIndex)
	}
	if !strings.Contains(strings.ToLower(hits[0].Snippet), "оглавл") {
		t.Fatalf("snippet: %s", hits[0].Snippet)
	}
	none, err := book.Search("xyzzy", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("unexpected: %#v", none)
	}
}
