package epub

import (
	"io"
	"path"
	"strings"
	"sync"
)

type Book struct {
	Path       string
	Key        string
	Title      string
	Author     string
	Language   string
	Identifier string
	CoverHref  string
	Format     string
	Chapters   []Chapter
	TOC        []TOCItem

	mu     sync.Mutex
	zr     zipReader
	closer io.Closer
	files  map[string]*zipFile
	plain  []string
}

type Chapter struct {
	ID    string `json:"id"`
	Href  string `json:"href"`
	Title string `json:"title"`
}

type TOCItem struct {
	Title        string    `json:"title"`
	Href         string    `json:"href"`
	Fragment     string    `json:"fragment"`
	ChapterIndex int       `json:"chapterIndex"`
	Children     []TOCItem `json:"children"`
}

func (b *Book) Close() error {
	if b == nil || b.closer == nil {
		return nil
	}
	return b.closer.Close()
}

func (b *Book) Resource(name string) ([]byte, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	name = normZipPath(name)
	f, ok := b.files[name]
	if !ok {
		return nil, "", errNotFound(name)
	}
	data, err := f.read()
	if err != nil {
		return nil, "", err
	}
	return data, mimeFor(name, f.contentType), nil
}

func (b *Book) ChapterHTML(index int) (string, error) {
	if index < 0 || index >= len(b.Chapters) {
		return "", errNotFound("chapter")
	}
	ch := b.Chapters[index]
	b.mu.Lock()
	f, ok := b.files[ch.Href]
	b.mu.Unlock()
	if !ok {
		return "", errNotFound(ch.Href)
	}
	data, err := f.read()
	if err != nil {
		return "", err
	}
	return sanitizeChapter(data, ch.Href), nil
}

func (b *Book) ChapterIndexByHref(href string) int {
	href, _ = splitFragment(href)
	href = normZipPath(href)
	for i, ch := range b.Chapters {
		if ch.Href == href {
			return i
		}
	}
	return -1
}

func mimeFor(name, declared string) string {
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".css":
		return "text/css"
	case ".otf":
		return "font/otf"
	case ".ttf":
		return "font/ttf"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
