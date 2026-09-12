package epub

import (
	"bytes"
	"io"
)

func NewMemory(origin, format string) *Book {
	return &Book{
		Path:   origin,
		Format: format,
		files:  make(zipMap),
	}
}

func (b *Book) Put(name, contentType string, data []byte) {
	name = normZipPath(name)
	cp := bytes.Clone(data)
	b.files[name] = &zipFile{
		name:        name,
		contentType: contentType,
		open:        func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(cp)), nil },
	}
}

func (b *Book) AddChapter(id, href, title string, xhtml []byte) {
	b.Put(href, "application/xhtml+xml", xhtml)
	b.Chapters = append(b.Chapters, Chapter{ID: id, Href: href, Title: title})
}

func (b *Book) SetTOC(items []TOCItem) {
	b.TOC = bindTOC(items, b)
	fillChapterTitles(b)
}

func (b *Book) Finish(size int64) {
	if b.Title == "" {
		b.Title = fallbackTitle(b.Path)
	}
	b.Key = bookKey(b.Identifier, b.Title, b.Path, size)
}

func (b *Book) SetCloser(c io.Closer) {
	b.closer = c
}
