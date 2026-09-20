package epub

import (
	"io"
	"path"
	"strconv"
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

func ValidTOCKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	for _, part := range strings.Split(key, ".") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func WalkTOC(items []TOCItem, fn func(item TOCItem, key string)) {
	var walk func([]TOCItem, string)
	walk = func(items []TOCItem, prefix string) {
		for i, it := range items {
			key := strconv.Itoa(i)
			if prefix != "" {
				key = prefix + "." + key
			}
			fn(it, key)
			if len(it.Children) > 0 {
				walk(it.Children, key)
			}
		}
	}
	walk(items, "")
}

func LookupTOC(items []TOCItem, key string) (TOCItem, bool) {
	var found TOCItem
	ok := false
	WalkTOC(items, func(item TOCItem, k string) {
		if !ok && k == key {
			found = item
			ok = true
		}
	})
	return found, ok
}

func TOCKeysForChapter(items []TOCItem, index int) []string {
	var keys []string
	WalkTOC(items, func(item TOCItem, key string) {
		if item.ChapterIndex == index {
			keys = append(keys, key)
		}
	})
	return keys
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
