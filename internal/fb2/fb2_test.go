package fb2

import (
	"strings"
	"testing"
)

func TestSampleFB2(t *testing.T) {
	book, err := OpenBytes("demo.fb2", []byte(sampleFB2))
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "Летний сад" {
		t.Fatalf("title: %q", book.Title)
	}
	if book.Author != "Анна Речная" {
		t.Fatalf("author: %q", book.Author)
	}
	if book.Format != "fb2" {
		t.Fatalf("format: %q", book.Format)
	}
	if len(book.Chapters) < 2 {
		t.Fatalf("chapters: %d", len(book.Chapters))
	}
	if book.Chapters[0].Title != "Глава первая" {
		t.Fatalf("ch0: %q", book.Chapters[0].Title)
	}
	html, err := book.ChapterHTML(0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "липов") {
		t.Fatalf("text missing: %s", html)
	}
	if !strings.Contains(html, "<em>") {
		t.Fatalf("emphasis missing: %s", html)
	}
	if book.CoverHref == "" {
		t.Fatal("cover")
	}
	if _, _, err := book.Resource(book.CoverHref); err != nil {
		t.Fatal(err)
	}
	if book.ChapterIndexByHref("c0002.xhtml") < 0 {
		t.Fatal("notes/chapter href")
	}
}

const sampleFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
  <description>
    <title-info>
      <author><first-name>Анна</first-name><last-name>Речная</last-name></author>
      <book-title>Летний сад</book-title>
      <lang>ru</lang>
      <coverpage><image l:href="#cover.png"/></coverpage>
    </title-info>
    <document-info><id>urn:boo:fb2-demo</id></document-info>
  </description>
  <body>
    <section>
      <title><p>Глава первая</p></title>
      <p>В саду пахло <emphasis>липовым</emphasis> мёдом.</p>
      <p>Сноска<a l:href="#n1">1</a>.</p>
    </section>
    <section>
      <title><p>Глава вторая</p></title>
      <p>Вечер был тихий.</p>
      <empty-line/>
      <p>На этом рассказ кончается.</p>
    </section>
  </body>
  <body name="notes">
    <section id="n1">
      <title><p>1</p></title>
      <p>Примечание к первой главе.</p>
    </section>
  </body>
  <binary id="cover.png" content-type="image/png">iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==</binary>
</FictionBook>
`
