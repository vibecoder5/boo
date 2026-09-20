package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxUndoKeep = 50

func (a UndoAction) info() UndoInfo {
	return UndoInfo{
		ID:        a.ID,
		Kind:      a.Kind,
		Label:     a.Label,
		Detail:    a.Detail,
		BookKey:   a.BookKey,
		BookTitle: a.BookTitle,
		CreatedAt: a.CreatedAt,
	}
}

func undoID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("undo|%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) UndoLog() []UndoInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.undoInfosLocked()
}

func (s *Store) undoInfosLocked() []UndoInfo {
	src := s.ws().UndoLog
	if len(src) == 0 {
		return []UndoInfo{}
	}
	out := make([]UndoInfo, 0, len(src))
	for _, a := range src {
		out = append(out, a.info())
	}
	return out
}

func (s *Store) UndoLast() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.undoLastLocked()
}

func (s *Store) RestoreTo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return os.ErrNotExist
	}
	ws := s.ws()
	idx := -1
	for i, a := range ws.UndoLog {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return os.ErrNotExist
	}
	for i := 0; i <= idx; i++ {
		s.applyUndoLocked(ws.UndoLog[i])
	}
	ws.UndoLog = append([]UndoAction(nil), ws.UndoLog[idx+1:]...)
	if ws.UndoLog == nil {
		ws.UndoLog = []UndoAction{}
	}
	ws.UpdatedAt = time.Now()
	return s.save()
}

func (s *Store) undoLastLocked() error {
	ws := s.ws()
	if len(ws.UndoLog) == 0 {
		return os.ErrNotExist
	}
	s.applyUndoLocked(ws.UndoLog[0])
	ws.UndoLog = append([]UndoAction(nil), ws.UndoLog[1:]...)
	if ws.UndoLog == nil {
		ws.UndoLog = []UndoAction{}
	}
	ws.UpdatedAt = time.Now()
	return s.save()
}

func (s *Store) bookTitleLocked(key string) string {
	for _, e := range s.ws().Library {
		if e.Key == key {
			return e.Title
		}
	}
	return ""
}

func (s *Store) snapshotBookLocked(key string, entry Entry, shared bool) UndoAction {
	ws := s.ws()
	id := undoID()
	snap := BookSnapshot{Entry: entry}
	if p, ok := ws.Books[key]; ok {
		pcopy := p
		snap.Progress = &pcopy
	}
	for _, b := range ws.Bookmarks {
		if b.BookKey == key {
			snap.Bookmarks = append(snap.Bookmarks, b)
		}
	}
	for _, h := range ws.Highlights {
		if h.BookKey == key {
			snap.Highlights = append(snap.Highlights, h)
		}
	}
	for _, n := range ws.Notes {
		if n.BookKey == key {
			snap.Notes = append(snap.Notes, n)
		}
	}
	for _, t := range ws.Todos {
		if t.BookKey == key {
			snap.Todos = append(snap.Todos, t)
		}
	}
	for _, h := range ws.History {
		if h.BookKey == key {
			snap.History = append(snap.History, h)
		}
	}
	if fold := ws.TOCFold[key]; len(fold) > 0 {
		snap.TOCFold = append([]string(nil), fold...)
	}
	if ch := ws.ReadChapters[key]; len(ch) > 0 {
		snap.ReadChapters = append([]int(nil), ch...)
	}
	if toc := ws.ReadTOC[key]; len(toc) > 0 {
		snap.ReadTOC = append([]string(nil), toc...)
	}
	for _, list := range ws.Lists {
		for _, k := range list.BookKeys {
			if k == key {
				snap.ListIDs = append(snap.ListIDs, list.ID)
				break
			}
		}
	}
	if !shared {
		libDir := filepath.Join(s.dir, "library")
		if entry.Path != "" && strings.HasPrefix(entry.Path, libDir) {
			name := "book" + filepath.Ext(entry.Path)
			if err := s.moveToTrash(id, entry.Path, name); err == nil {
				snap.FileName = name
				snap.TrashRel = id
			}
		}
		if entry.Cover != "" {
			coverPath := filepath.Join(s.dir, "covers", filepath.Base(entry.Cover))
			name := "cover" + filepath.Ext(entry.Cover)
			if err := s.moveToTrash(id, coverPath, name); err == nil {
				snap.CoverName = name
				snap.TrashRel = id
			}
		}
	}
	title := entry.Title
	if title == "" {
		title = "Книга"
	}
	return UndoAction{
		ID:        id,
		Kind:      UndoDeleteBook,
		Label:     "Удалена книга",
		Detail:    title,
		BookKey:   key,
		BookTitle: entry.Title,
		Book:      &snap,
	}
}

func (s *Store) pushUndoLocked(a UndoAction) {
	if a.ID == "" {
		a.ID = undoID()
	}
	a.CreatedAt = time.Now()
	ws := s.ws()
	ws.UndoLog = append([]UndoAction{a}, ws.UndoLog...)
	for len(ws.UndoLog) > maxUndoKeep {
		old := ws.UndoLog[len(ws.UndoLog)-1]
		s.dropTrash(old)
		ws.UndoLog = ws.UndoLog[:len(ws.UndoLog)-1]
	}
	ws.UpdatedAt = a.CreatedAt
}

func (s *Store) applyUndoLocked(a UndoAction) {
	switch a.Kind {
	case UndoDeleteBook:
		s.restoreBookLocked(a)
	case UndoDeleteBookmark:
		if a.Bookmark == nil {
			return
		}
		ws := s.ws()
		for _, b := range ws.Bookmarks {
			if b.ID == a.Bookmark.ID {
				return
			}
		}
		ws.Bookmarks = append([]Bookmark{*a.Bookmark}, ws.Bookmarks...)
	case UndoDeleteNote:
		if a.Note == nil {
			return
		}
		ws := s.ws()
		for _, n := range ws.Notes {
			if n.ID == a.Note.ID {
				return
			}
		}
		ws.Notes = append([]Note{*a.Note}, ws.Notes...)
	}
}

func (s *Store) restoreBookLocked(a UndoAction) {
	if a.Book == nil {
		return
	}
	snap := a.Book
	entry := snap.Entry
	if snap.TrashRel != "" {
		if snap.FileName != "" && entry.Path != "" {
			_ = s.restoreFromTrash(snap.TrashRel, snap.FileName, entry.Path)
		}
		if snap.CoverName != "" && entry.Cover != "" {
			dest := filepath.Join(s.dir, "covers", filepath.Base(entry.Cover))
			_ = s.restoreFromTrash(snap.TrashRel, snap.CoverName, dest)
		}
		s.removeTrashDir(snap.TrashRel)
	}
	ws := s.ws()
	if !libraryHas(ws, entry.Key) {
		out := []Entry{entry}
		ws.Library = append(out, ws.Library...)
	}
	if snap.Progress != nil {
		if ws.Books == nil {
			ws.Books = map[string]Progress{}
		}
		if _, ok := ws.Books[entry.Key]; !ok {
			ws.Books[entry.Key] = *snap.Progress
		}
	}
	ws.Bookmarks = mergeBookmarks(ws.Bookmarks, snap.Bookmarks)
	ws.Highlights = mergeHighlights(ws.Highlights, snap.Highlights)
	ws.Notes = mergeNotes(ws.Notes, snap.Notes)
	ws.Todos = mergeTodos(ws.Todos, snap.Todos)
	ws.History = mergeHistory(ws.History, snap.History)
	if len(snap.TOCFold) > 0 {
		if ws.TOCFold == nil {
			ws.TOCFold = map[string][]string{}
		}
		if len(ws.TOCFold[entry.Key]) == 0 {
			ws.TOCFold[entry.Key] = append([]string(nil), snap.TOCFold...)
		}
	}
	if len(snap.ReadChapters) > 0 {
		if ws.ReadChapters == nil {
			ws.ReadChapters = map[string][]int{}
		}
		if len(ws.ReadChapters[entry.Key]) == 0 {
			ws.ReadChapters[entry.Key] = append([]int(nil), snap.ReadChapters...)
		}
	}
	if len(snap.ReadTOC) > 0 {
		if ws.ReadTOC == nil {
			ws.ReadTOC = map[string][]string{}
		}
		if len(ws.ReadTOC[entry.Key]) == 0 {
			ws.ReadTOC[entry.Key] = append([]string(nil), snap.ReadTOC...)
		}
	}
	for _, listID := range snap.ListIDs {
		for i := range ws.Lists {
			if ws.Lists[i].ID != listID {
				continue
			}
			already := false
			for _, k := range ws.Lists[i].BookKeys {
				if k == entry.Key {
					already = true
					break
				}
			}
			if !already {
				ws.Lists[i].BookKeys = append(ws.Lists[i].BookKeys, entry.Key)
			}
		}
	}
}

func mergeBookmarks(dst, src []Bookmark) []Bookmark {
	have := map[string]bool{}
	for _, b := range dst {
		have[b.ID] = true
	}
	for i := len(src) - 1; i >= 0; i-- {
		if have[src[i].ID] {
			continue
		}
		dst = append([]Bookmark{src[i]}, dst...)
	}
	return dst
}

func mergeHighlights(dst, src []Highlight) []Highlight {
	have := map[string]bool{}
	for _, h := range dst {
		have[h.ID] = true
	}
	for i := len(src) - 1; i >= 0; i-- {
		if have[src[i].ID] {
			continue
		}
		dst = append([]Highlight{src[i]}, dst...)
	}
	return dst
}

func mergeNotes(dst, src []Note) []Note {
	have := map[string]bool{}
	for _, n := range dst {
		have[n.ID] = true
	}
	for i := len(src) - 1; i >= 0; i-- {
		if have[src[i].ID] {
			continue
		}
		dst = append([]Note{src[i]}, dst...)
	}
	return dst
}

func mergeTodos(dst, src []Todo) []Todo {
	have := map[string]bool{}
	for _, t := range dst {
		have[t.ID] = true
	}
	for _, t := range src {
		if have[t.ID] {
			continue
		}
		dst = append(dst, t)
	}
	return dst
}

func mergeHistory(dst, src []HistoryEntry) []HistoryEntry {
	have := map[string]bool{}
	for _, h := range dst {
		have[h.ID] = true
	}
	var add []HistoryEntry
	for _, h := range src {
		if have[h.ID] {
			continue
		}
		add = append(add, h)
	}
	if len(add) == 0 {
		return dst
	}
	out := append(append([]HistoryEntry{}, add...), dst...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > maxHistoryKeep {
		out = out[:maxHistoryKeep]
	}
	return out
}

func (s *Store) trashDir(id string) string {
	return filepath.Join(s.dir, "trash", id)
}

func (s *Store) dropTrash(a UndoAction) {
	if a.ID != "" {
		s.removeTrashDir(a.ID)
	}
	if a.Book != nil && a.Book.TrashRel != "" && a.Book.TrashRel != a.ID {
		s.removeTrashDir(a.Book.TrashRel)
	}
}

func (s *Store) removeTrashDir(id string) {
	if id == "" || id == "." || id == ".." {
		return
	}
	_ = os.RemoveAll(s.trashDir(id))
}

func (s *Store) moveToTrash(id, src, destName string) error {
	if src == "" || destName == "" || destName == "." || destName == ".." {
		return os.ErrInvalid
	}
	if _, err := os.Stat(src); err != nil {
		return err
	}
	destDir := s.trashDir(id)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, filepath.Base(destName))
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

func (s *Store) restoreFromTrash(id, destName, destPath string) error {
	if destPath == "" || destName == "" {
		return os.ErrInvalid
	}
	src := filepath.Join(s.trashDir(id), filepath.Base(destName))
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, destPath); err == nil {
		return nil
	}
	if err := copyFile(src, destPath); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
