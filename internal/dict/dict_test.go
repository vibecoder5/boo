package dict

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestParseTSVAndLookup(t *testing.T) {
	idx, err := Parse("en-ru.txt", []byte("hello\tпривет\nworld\tмир\nrun\tбежать\nlook up\tискать\n"))
	if err != nil {
		t.Fatal(err)
	}
	if idx.FromLang != "en" || idx.ToLang != "ru" {
		t.Fatalf("langs %s %s", idx.FromLang, idx.ToLang)
	}
	hits := idx.Lookup("Hello!")
	if len(hits) != 1 || hits[0].Text != "привет" {
		t.Fatalf("hello %#v", hits)
	}
	if got := idx.Lookup("running"); len(got) != 1 || got[0].Text != "бежать" {
		t.Fatalf("run via running %#v", got)
	}
	if got := idx.Lookup("look up"); len(got) != 1 || got[0].Text != "искать" {
		t.Fatalf("look up %#v", got)
	}
	if idx.Lookup("missing") != nil {
		t.Fatal("missing should be empty")
	}
}

func TestParseDSL(t *testing.T) {
	src := `#NAME "Lingvo (En-Ru)"
#INDEX_LANGUAGE "English"
#CONTENTS_LANGUAGE "Russian"

apple
	[p][i]n[/i][/p]
	яблоко

col{o}ur
	цвет
`
	idx, err := Parse("dict.dsl", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if idx.Name != "Lingvo (En-Ru)" || idx.FromLang != "en" || idx.ToLang != "ru" {
		t.Fatalf("meta %#v", idx)
	}
	if got := idx.Lookup("apples"); len(got) != 1 || !strings.Contains(got[0].Text, "яблоко") {
		t.Fatalf("apple %#v", got)
	}
	if got := idx.Lookup("colour"); len(got) != 1 {
		t.Fatalf("colour %#v", got)
	}
}

func TestParseJSONAndXDXF(t *testing.T) {
	idx, err := Parse("ru-en.json", []byte(`{"книга":"book","текст":["text","passage"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if idx.FromLang != "ru" || idx.ToLang != "en" {
		t.Fatalf("langs %s %s", idx.FromLang, idx.ToLang)
	}
	got := idx.Lookup("книге")
	if len(got) != 1 || got[0].Text != "book" {
		t.Fatalf("книга %#v", got)
	}
	xdxf, err := Parse("mini.xdxf", []byte(`<xdxf><ar><k>world</k>мир</ar></xdxf>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := xdxf.Lookup("world"); len(got) != 1 || got[0].Text != "мир" {
		t.Fatalf("xdxf %#v", got)
	}
}

func TestParseUTF16DSL(t *testing.T) {
	src := "#NAME \"En-Ru\"\n#INDEX_LANGUAGE \"English\"\n#CONTENTS_LANGUAGE \"Russian\"\n\nhello\n\tпривет\n"
	u := utf16.Encode([]rune(src))
	data := []byte{0xFF, 0xFE}
	for _, r := range u {
		data = append(data, byte(r), byte(r>>8))
	}
	idx, err := Parse("en-ru.dsl", data)
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.Lookup("hello"); len(got) != 1 || got[0].Text != "привет" {
		t.Fatalf("%#v", got)
	}
}

func TestEmptyRejected(t *testing.T) {
	if _, err := Parse("empty.txt", []byte("# just a comment\n")); err == nil {
		t.Fatal("expected error")
	}
}
