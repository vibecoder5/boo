package open

import (
	"testing"

	"boo/internal/epub"
)

func TestOpenBytesEPUBAndFB2(t *testing.T) {
	epubData, err := epub.Sample()
	if err != nil {
		t.Fatal(err)
	}
	book, err := OpenBytes("demo.epub", epubData)
	if err != nil {
		t.Fatal(err)
	}
	if book.Format != "epub" {
		t.Fatalf("epub format: %q", book.Format)
	}

	book, err = OpenBytes("demo.fb2", []byte(fb2Sample))
	if err != nil {
		t.Fatal(err)
	}
	if book.Format != "fb2" || book.Title != "Летний сад" {
		t.Fatalf("fb2: %q %q", book.Format, book.Title)
	}

	book, err = OpenBytes("notes.txt", []byte("Глава 1\nРаз.\n\nГлава 2\nДва.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if book.Format != "txt" || len(book.Chapters) != 2 {
		t.Fatalf("txt: %q %d", book.Format, len(book.Chapters))
	}
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
