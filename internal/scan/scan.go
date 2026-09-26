package scan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"boo/internal/epub"
	"boo/internal/open"
)

const (
	MaxBooks     = 400
	MaxDepth     = 8
	MaxFileBytes = 256 << 20
)

var errStop = errors.New("stop")

type Hit struct {
	Path     string
	Name     string
	Title    string
	Author   string
	Format   string
	Key      string
	ChapterN int
}

type Result struct {
	Root       string
	Books      []Hit
	Truncated  bool
	Unreadable int
}

func Folder(root string) (Result, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("папка не найдена")
	}
	var books []Hit
	unreadable := 0
	truncated := false
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return fs.SkipDir
			}
			if path != root && dirDepth(root, path) > MaxDepth {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		kind, strict := classify(d.Name())
		if kind == "" {
			return nil
		}
		if len(books) >= MaxBooks {
			truncated = true
			return errStop
		}
		book, err := Open(path)
		if err != nil {
			if strict {
				unreadable++
			}
			return nil
		}
		title := strings.TrimSpace(book.Title)
		if title == "" {
			title = strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		}
		books = append(books, Hit{
			Path:     path,
			Name:     d.Name(),
			Title:    title,
			Author:   strings.TrimSpace(book.Author),
			Format:   book.Format,
			Key:      book.Key,
			ChapterN: len(book.Chapters),
		})
		_ = book.Close()
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errStop) {
		return Result{}, walkErr
	}
	sort.Slice(books, func(i, j int) bool {
		ti := strings.ToLower(books[i].Title)
		tj := strings.ToLower(books[j].Title)
		if ti != tj {
			return ti < tj
		}
		return books[i].Path < books[j].Path
	})
	if books == nil {
		books = []Hit{}
	}
	return Result{Root: root, Books: books, Truncated: truncated, Unreadable: unreadable}, nil
}

func Open(path string) (*epub.Book, error) {
	if !accepts(filepath.Base(path)) {
		return nil, fmt.Errorf("нужен EPUB или FB2")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("файл не найден")
	}
	if info.Size() > MaxFileBytes {
		return nil, fmt.Errorf("файл слишком большой")
	}
	book, err := open.Open(path)
	if err != nil {
		return nil, err
	}
	if book.Format != "epub" && book.Format != "fb2" {
		_ = book.Close()
		return nil, fmt.Errorf("нужен EPUB или FB2")
	}
	return book, nil
}

func accepts(name string) bool {
	kind, _ := classify(name)
	return kind != ""
}

func classify(name string) (kind string, strict bool) {
	n := strings.ToLower(name)
	switch {
	case strings.HasSuffix(n, ".txt"), strings.HasSuffix(n, ".text"), strings.HasSuffix(n, ".md"), strings.HasSuffix(n, ".markdown"):
		return "", false
	case strings.HasSuffix(n, ".epub"):
		return "epub", true
	case strings.HasSuffix(n, ".fb2"):
		return "fb2", true
	case strings.HasSuffix(n, ".fb2.zip"):
		return "zip", true
	case strings.HasSuffix(n, ".zip"):
		return "zip", false
	default:
		return "", false
	}
}

func skipDir(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "$recycle.bin", "system volume information":
		return true
	default:
		return false
	}
}

func dirDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == "" {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}
