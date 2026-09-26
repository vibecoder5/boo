package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"boo/internal/desktop"
	"boo/internal/epub"
	"boo/internal/scan"
	"boo/internal/store"
)

func (s *Server) handleScanFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	root := strings.TrimSpace(body.Path)
	if root == "" {
		if !desktop.CanPickFolder() {
			http.Error(w, "укажите путь к папке", http.StatusBadRequest)
			return
		}
		picked, ok, err := desktop.PickFolder("Выберите папку с книгами")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			writeJSON(w, map[string]any{"cancelled": true})
			return
		}
		root = picked
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		http.Error(w, "папка не найдена", http.StatusBadRequest)
		return
	}
	result, err := scan.Folder(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := s.store.UI().ImportWorkspace
	ws, ok := s.store.WorkspaceByName(name)
	if ok {
		name = ws.Name
	}
	books := make([]map[string]any, 0, len(result.Books))
	for _, hit := range result.Books {
		inLibrary := false
		if ok {
			inLibrary = s.store.HasBook(ws.ID, hit.Key, hit.Title)
		}
		books = append(books, map[string]any{
			"path":      hit.Path,
			"name":      hit.Name,
			"title":     hit.Title,
			"author":    hit.Author,
			"format":    hit.Format,
			"inLibrary": inLibrary,
		})
	}
	writeJSON(w, map[string]any{
		"root":       result.Root,
		"workspace":  name,
		"truncated":  result.Truncated,
		"unreadable": result.Unreadable,
		"books":      books,
	})
}

func (s *Server) handleScanAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if len(body.Paths) == 0 {
		http.Error(w, "ничего не выбрано", http.StatusBadRequest)
		return
	}
	if len(body.Paths) > scan.MaxBooks {
		body.Paths = body.Paths[:scan.MaxBooks]
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	added, skipped, failed, wsName, err := s.importScanned(body.Paths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"added":     added,
		"skipped":   skipped,
		"failed":    failed,
		"workspace": wsName,
	})
}

func (s *Server) importScanned(paths []string) (added, skipped, failed int, wsName string, err error) {
	info, err := s.store.EnsureNamedWorkspace(s.store.UI().ImportWorkspace)
	if err != nil {
		return 0, 0, 0, "", err
	}
	wsName = info.Name
	seen := map[string]bool{}
	var entries []store.Entry
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			failed++
			continue
		}
		keyPath := abs
		if runtime.GOOS == "windows" {
			keyPath = strings.ToLower(abs)
		}
		if seen[keyPath] {
			continue
		}
		seen[keyPath] = true
		book, err := scan.Open(abs)
		if err != nil {
			failed++
			continue
		}
		if s.store.HasBook(info.ID, book.Key, book.Title) {
			_ = book.Close()
			skipped++
			continue
		}
		entry, err := s.entryFromScan(book, abs)
		_ = book.Close()
		if err != nil {
			failed++
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return 0, skipped, failed, wsName, nil
	}
	n, skip2, err := s.store.AddToWorkspace(info.ID, entries)
	if err != nil {
		return 0, skipped, failed, wsName, err
	}
	return n, skipped + skip2, failed, wsName, nil
}

func (s *Server) entryFromScan(book *epub.Book, src string) (store.Entry, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return store.Entry{}, err
	}
	saved, err := s.store.SaveBookFile(book.Key, filepath.Base(src), data)
	if err != nil {
		return store.Entry{}, err
	}
	cover := ""
	if book.CoverHref != "" {
		if cdata, mime, err := book.Resource(book.CoverHref); err == nil {
			if name, err := s.store.SaveCover(book.Key, mime, cdata); err == nil {
				cover = name
			}
		}
	}
	format := book.Format
	if format == "" {
		format = bookFormat(book)
	}
	return store.Entry{
		Key:      book.Key,
		Title:    book.Title,
		Author:   book.Author,
		Path:     saved,
		Format:   format,
		Cover:    cover,
		ChapterN: len(book.Chapters),
	}, nil
}
