package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"boo/docs"
	"boo/internal/dict"
	"boo/internal/drive"
	"boo/internal/epub"
	"boo/internal/open"
	"boo/internal/store"
)

type Server struct {
	mu     sync.Mutex
	book   *epub.Book
	store  *store.Store
	drive  *drive.Service
	ui     fs.FS
	dictMu sync.Mutex
	dicts  map[string]*dict.Index
}

func New(st *store.Store, ui fs.FS, book *epub.Book) *Server {
	s := &Server{store: st, drive: drive.New(st.Dir()), ui: ui, book: book, dicts: map[string]*dict.Index{}}
	s.repairSharedCovers()
	if book != nil {
		_ = s.remember(book)
	}
	return s
}

func (s *Server) Listen(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/chapter", s.handleChapter)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("PUT /api/progress", s.handleSaveProgress)
	mux.HandleFunc("POST /api/progress", s.handleSaveProgress)
	mux.HandleFunc("POST /api/bookmarks", s.handleAddBookmark)
	mux.HandleFunc("DELETE /api/bookmarks", s.handleDeleteBookmark)
	mux.HandleFunc("POST /api/highlights", s.handleAddHighlight)
	mux.HandleFunc("PUT /api/highlights", s.handleUpdateHighlight)
	mux.HandleFunc("DELETE /api/highlights", s.handleDeleteHighlight)
	mux.HandleFunc("POST /api/notes", s.handleAddNote)
	mux.HandleFunc("PUT /api/notes", s.handleUpdateNote)
	mux.HandleFunc("DELETE /api/notes", s.handleDeleteNote)
	mux.HandleFunc("GET /api/todos", s.handleTodos)
	mux.HandleFunc("POST /api/todos", s.handleAddTodo)
	mux.HandleFunc("PUT /api/todos", s.handleUpdateTodo)
	mux.HandleFunc("DELETE /api/todos", s.handleDeleteTodo)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/history", s.handleAddHistory)
	mux.HandleFunc("POST /api/history/session", s.handleHistorySession)
	mux.HandleFunc("POST /api/history/read-time", s.handleHistoryReadTime)
	mux.HandleFunc("GET /api/undo", s.handleUndoLog)
	mux.HandleFunc("POST /api/undo", s.handleUndoLast)
	mux.HandleFunc("POST /api/undo/restore", s.handleUndoRestore)
	mux.HandleFunc("PUT /api/ui", s.handleSaveUI)
	mux.HandleFunc("GET /api/ui/background", s.handleWelcomeBackground)
	mux.HandleFunc("POST /api/ui/background", s.handleSaveWelcomeBackground)
	mux.HandleFunc("DELETE /api/ui/background", s.handleDeleteWelcomeBackground)
	mux.HandleFunc("PUT /api/toc-fold", s.handleSaveTOCFold)
	mux.HandleFunc("PUT /api/chapters/read", s.handleSetChapterRead)
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.HandleFunc("POST /api/demo", s.handleDemo)
	mux.HandleFunc("POST /api/guide", s.handleGuide)
	mux.HandleFunc("POST /api/close", s.handleClose)
	mux.HandleFunc("POST /api/workspaces", s.handleCreateWorkspace)
	mux.HandleFunc("PUT /api/workspaces", s.handleRenameWorkspace)
	mux.HandleFunc("PUT /api/workspaces/current", s.handleSwitchWorkspace)
	mux.HandleFunc("PUT /api/workspaces/order", s.handleReorderWorkspaces)
	mux.HandleFunc("PUT /api/workspaces/pin", s.handlePinWorkspace)
	mux.HandleFunc("POST /api/library/move", s.handleMoveBook)
	mux.HandleFunc("POST /api/lists", s.handleCreateList)
	mux.HandleFunc("DELETE /api/lists", s.handleDeleteList)
	mux.HandleFunc("POST /api/lists/books", s.handleAddToList)
	mux.HandleFunc("DELETE /api/lists/books", s.handleRemoveFromList)
	mux.HandleFunc("POST /api/library/open", s.handleLibraryOpen)
	mux.HandleFunc("PUT /api/library/finished", s.handleSetFinished)
	mux.HandleFunc("PUT /api/library/meta", s.handleSetBookMeta)
	mux.HandleFunc("POST /api/library/meta", s.handleSetBookMeta)
	mux.HandleFunc("PUT /api/library/dictionary", s.handleSetBookDictionary)
	mux.HandleFunc("GET /api/dictionaries", s.handleDictionaries)
	mux.HandleFunc("POST /api/dictionaries", s.handleAddDictionary)
	mux.HandleFunc("DELETE /api/dictionaries", s.handleDeleteDictionary)
	mux.HandleFunc("GET /api/dictionaries/lookup", s.handleDictionaryLookup)
	mux.HandleFunc("DELETE /api/library", s.handleLibraryDelete)
	mux.HandleFunc("GET /api/library/cover", s.handleLibraryCover)
	mux.HandleFunc("GET /api/library/search", s.handleLibrarySearch)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("POST /api/import", s.handleImport)
	mux.HandleFunc("GET /api/drive", s.handleDriveStatus)
	mux.HandleFunc("POST /api/drive/credentials", s.handleDriveCredentials)
	mux.HandleFunc("POST /api/drive/connect", s.handleDriveConnect)
	mux.HandleFunc("GET /api/drive/callback", s.handleDriveCallback)
	mux.HandleFunc("POST /api/drive/sync", s.handleDriveSync)
	mux.HandleFunc("POST /api/drive/disconnect", s.handleDriveDisconnect)
	mux.HandleFunc("GET /res", s.handleResource)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.ui))))
	return withLocalhost(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.ui, "index.html")
	if err != nil {
		http.Error(w, "ui missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	book := s.book
	s.mu.Unlock()

	payload := map[string]any{
		"book":          s.bookPayload(book),
		"ui":            s.store.UI(),
		"progress":      nil,
		"workspace":     s.store.CurrentWorkspace(),
		"workspaces":    s.store.Workspaces(),
		"library":       s.libraryPayload(),
		"lists":         s.store.Lists(),
		"bookmarks":     []store.Bookmark{},
		"highlights":    []store.Highlight{},
		"notes":         []store.Note{},
		"todos":         []store.Todo{},
		"todoBook":      nil,
		"history":       s.store.History(10),
		"undo":          s.store.UndoLog(),
		"readStats":     s.store.ReadStats(""),
		"bookReadStats": nil,
		"tocFold":       []string{},
		"readChapters":  []int{},
		"readTOC":       []string{},
		"dictionaries":  s.store.Dictionaries(),
	}
	if s.drive != nil {
		payload["drive"] = s.drive.Status()
	}
	if book != nil {
		if p, ok := s.store.Progress(book.Key); ok {
			payload["progress"] = p
		}
		payload["bookmarks"] = s.store.Bookmarks(book.Key)
		payload["highlights"] = s.store.Highlights(book.Key)
		payload["notes"] = s.store.Notes(book.Key)
		payload["tocFold"] = s.store.TOCFold(book.Key)
		payload["readChapters"] = s.store.ReadChapters(book.Key)
		payload["readTOC"] = s.store.ReadTOC(book.Key)
		payload["bookReadStats"] = s.store.ReadStats(book.Key)
	}
	todoBook, todos := s.todoState(book)
	payload["todoBook"] = todoBook
	payload["todos"] = todos
	writeJSON(w, payload)
}

func (s *Server) todoState(book *epub.Book) (map[string]any, []store.Todo) {
	key, title := "", ""
	if book != nil {
		key = book.Key
		title = book.Title
	} else if items := s.store.Library(); len(items) > 0 {
		key = items[0].Key
		title = items[0].Title
	}
	if key == "" {
		return nil, []store.Todo{}
	}
	return map[string]any{"key": key, "title": title}, s.store.Todos(key)
}

func (s *Server) resolveTodoBook(key string) (string, string, error) {
	key = strings.TrimSpace(key)
	if key != "" {
		if e, ok := s.store.Entry(key); ok {
			return e.Key, e.Title, nil
		}
		if book := s.current(); book != nil && book.Key == key {
			return book.Key, book.Title, nil
		}
		return "", "", os.ErrNotExist
	}
	if book := s.current(); book != nil {
		return book.Key, book.Title, nil
	}
	if items := s.store.Library(); len(items) > 0 {
		return items[0].Key, items[0].Title, nil
	}
	return "", "", os.ErrNotExist
}

func (s *Server) writeTodos(w http.ResponseWriter, key, title string) {
	writeJSON(w, map[string]any{
		"todoBook": map[string]any{"key": key, "title": title},
		"todos":    s.store.Todos(key),
	})
}

func (s *Server) handleTodos(w http.ResponseWriter, r *http.Request) {
	key, title, err := s.resolveTodoBook(r.URL.Query().Get("key"))
	if err != nil {
		http.Error(w, "книга не найдена", http.StatusNotFound)
		return
	}
	s.writeTodos(w, key, title)
}

func (s *Server) handleAddTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Text  string `json:"text"`
		DueAt string `json:"dueAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key, title, err := s.resolveTodoBook(body.Key)
	if err != nil {
		http.Error(w, "книга не найдена", http.StatusNotFound)
		return
	}
	dueAt, err := store.ParseDueAt(body.DueAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.AddTodo(store.Todo{BookKey: key, Text: body.Text, DueAt: dueAt}); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "книга не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeTodos(w, key, title)
}

func (s *Server) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Key   string `json:"key"`
		Text  string `json:"text"`
		DueAt string `json:"dueAt"`
		Done  bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	dueAt, err := store.ParseDueAt(body.DueAt)
	if err != nil && strings.TrimSpace(body.DueAt) != "" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := s.store.UpdateTodo(body.ID, body.Text, dueAt, body.Done)
	if err != nil {
		http.Error(w, "задание не найдено", http.StatusNotFound)
		return
	}
	key, title := item.BookKey, ""
	if e, ok := s.store.Entry(item.BookKey); ok {
		title = e.Title
	} else if book := s.current(); book != nil && book.Key == item.BookKey {
		title = book.Title
	}
	if want := strings.TrimSpace(body.Key); want != "" && want != key {
		http.Error(w, "задание не найдено", http.StatusNotFound)
		return
	}
	s.writeTodos(w, key, title)
}

func (s *Server) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	keyHint := strings.TrimSpace(r.URL.Query().Get("key"))
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	key, title := keyHint, ""
	if key == "" {
		if book := s.current(); book != nil {
			key = book.Key
			title = book.Title
		}
	}
	if e, ok := s.store.Entry(key); ok {
		title = e.Title
		key = e.Key
	}
	if err := s.store.RemoveTodo(id); err != nil {
		http.Error(w, "задание не найдено", http.StatusNotFound)
		return
	}
	if key == "" {
		if items := s.store.Library(); len(items) > 0 {
			key = items[0].Key
			title = items[0].Title
		}
	}
	if key == "" {
		writeJSON(w, map[string]any{"todoBook": nil, "todos": []store.Todo{}})
		return
	}
	s.writeTodos(w, key, title)
}

func (s *Server) writeHistory(w http.ResponseWriter) {
	payload := map[string]any{
		"history":       s.store.History(10),
		"readStats":     s.store.ReadStats(""),
		"bookReadStats": nil,
	}
	if book := s.current(); book != nil {
		payload["bookReadStats"] = s.store.ReadStats(book.Key)
	}
	writeJSON(w, payload)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.writeHistory(w)
}

func (s *Server) historyProgress(book *epub.Book, chapterIndex int, scrollRatio float64) (int, float64, string) {
	p, ok := s.store.Progress(book.Key)
	if chapterIndex < 0 {
		if ok {
			chapterIndex = p.ChapterIndex
		} else {
			chapterIndex = 0
		}
	}
	if chapterIndex < 0 {
		chapterIndex = 0
	}
	if chapterIndex >= len(book.Chapters) {
		chapterIndex = len(book.Chapters) - 1
	}
	if chapterIndex < 0 {
		chapterIndex = 0
	}
	if scrollRatio < 0 {
		if ok {
			scrollRatio = p.ScrollRatio
		} else {
			scrollRatio = 0
		}
	}
	if scrollRatio > 1 {
		scrollRatio = 1
	}
	title := ""
	if chapterIndex >= 0 && chapterIndex < len(book.Chapters) {
		title = book.Chapters[chapterIndex].Title
	}
	return chapterIndex, scrollRatio, title
}

func (s *Server) RecordSession() {
	s.recordSession(-1, -1)
}

func (s *Server) recordSession(chapterIndex int, scrollRatio float64) {
	book := s.current()
	if book == nil {
		return
	}
	chapterIndex, scrollRatio, chapterTitle := s.historyProgress(book, chapterIndex, scrollRatio)
	if chapterIndex >= 0 && chapterIndex < len(book.Chapters) {
		_ = s.store.SaveProgress(book.Key, store.Progress{
			Title:        book.Title,
			Author:       book.Author,
			ChapterIndex: chapterIndex,
			ScrollRatio:  scrollRatio,
		})
	}
	_, _ = s.store.AddHistory(store.HistoryEntry{
		BookKey:      book.Key,
		Title:        book.Title,
		Author:       book.Author,
		Kind:         store.HistorySession,
		ChapterIndex: chapterIndex,
		ChapterTitle: chapterTitle,
		ScrollRatio:  scrollRatio,
	})
}

func (s *Server) handleAddHistory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key          string   `json:"key"`
		Text         string   `json:"text"`
		ChapterIndex *int     `json:"chapterIndex"`
		ScrollRatio  *float64 `json:"scrollRatio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key, title, err := s.resolveTodoBook(body.Key)
	if err != nil {
		http.Error(w, "книга не найдена", http.StatusNotFound)
		return
	}
	chapterIndex, chapterTitle := 0, ""
	scrollRatio := 0.0
	wantChapter, wantScroll := -1, -1.0
	if body.ChapterIndex != nil {
		wantChapter = *body.ChapterIndex
	}
	if body.ScrollRatio != nil {
		wantScroll = *body.ScrollRatio
	}
	if book := s.current(); book != nil && book.Key == key {
		chapterIndex, scrollRatio, chapterTitle = s.historyProgress(book, wantChapter, wantScroll)
	} else if e, ok := s.store.Entry(key); ok {
		chapterIndex = e.ChapterIndex
		scrollRatio = e.ScrollRatio
	}
	author := ""
	if e, ok := s.store.Entry(key); ok {
		if title == "" {
			title = e.Title
		}
		author = e.Author
	}
	if _, err := s.store.AddHistory(store.HistoryEntry{
		BookKey:      key,
		Title:        title,
		Author:       author,
		Kind:         store.HistoryNote,
		Text:         body.Text,
		ChapterIndex: chapterIndex,
		ChapterTitle: chapterTitle,
		ScrollRatio:  scrollRatio,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeHistory(w)
}

func (s *Server) handleHistoryReadTime(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key          string   `json:"key"`
		DurationSec  int      `json:"durationSec"`
		ChapterIndex *int     `json:"chapterIndex"`
		ScrollRatio  *float64 `json:"scrollRatio"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
	}
	key := strings.TrimSpace(body.Key)
	title, author := "", ""
	if key != "" {
		resolved, name, err := s.resolveTodoBook(key)
		if err != nil {
			http.Error(w, "книга не найдена", http.StatusNotFound)
			return
		}
		key, title = resolved, name
	} else if book := s.current(); book != nil {
		key, title, author = book.Key, book.Title, book.Author
	} else {
		http.Error(w, "книга нужна", http.StatusBadRequest)
		return
	}
	chapterIndex, chapterTitle := 0, ""
	scrollRatio := 0.0
	wantChapter, wantScroll := -1, -1.0
	if body.ChapterIndex != nil {
		wantChapter = *body.ChapterIndex
	}
	if body.ScrollRatio != nil {
		wantScroll = *body.ScrollRatio
	}
	if book := s.current(); book != nil && book.Key == key {
		chapterIndex, scrollRatio, chapterTitle = s.historyProgress(book, wantChapter, wantScroll)
	} else if e, ok := s.store.Entry(key); ok {
		chapterIndex = e.ChapterIndex
		scrollRatio = e.ScrollRatio
		if title == "" {
			title = e.Title
		}
		if author == "" {
			author = e.Author
		}
	}
	if e, ok := s.store.Entry(key); ok {
		if title == "" {
			title = e.Title
		}
		if author == "" {
			author = e.Author
		}
	}
	if _, err := s.store.AddHistory(store.HistoryEntry{
		BookKey:      key,
		Title:        title,
		Author:       author,
		Kind:         store.HistoryReadTime,
		DurationSec:  body.DurationSec,
		ChapterIndex: chapterIndex,
		ChapterTitle: chapterTitle,
		ScrollRatio:  scrollRatio,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeHistory(w)
}

func (s *Server) handleUndoLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"undo": s.store.UndoLog()})
}

func (s *Server) handleUndoLast(w http.ResponseWriter, r *http.Request) {
	if err := s.store.UndoLast(); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "нечего отменять", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleUndoRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.RestoreTo(body.ID); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "запись не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleHistorySession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChapterIndex *int     `json:"chapterIndex"`
		ScrollRatio  *float64 `json:"scrollRatio"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if s.current() == nil {
		s.writeHistory(w)
		return
	}
	chapterIndex, scrollRatio := -1, -1.0
	if body.ChapterIndex != nil {
		chapterIndex = *body.ChapterIndex
	}
	if body.ScrollRatio != nil {
		scrollRatio = *body.ScrollRatio
	}
	s.recordSession(chapterIndex, scrollRatio)
	s.writeHistory(w)
}

func (s *Server) handleChapter(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	html, err := book.ChapterHTML(i)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	title := ""
	if i >= 0 && i < len(book.Chapters) {
		title = book.Chapters[i].Title
	}
	writeJSON(w, map[string]any{
		"index": i,
		"title": title,
		"html":  html,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) > 80 {
		q = string([]rune(q)[:80])
	}
	hits, err := book.Search(q, 40)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hits == nil {
		hits = []epub.Hit{}
	}
	writeJSON(w, map[string]any{"query": q, "hits": hits})
}

func (s *Server) handleSaveProgress(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		ChapterIndex int     `json:"chapterIndex"`
		ScrollRatio  float64 `json:"scrollRatio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ChapterIndex < 0 || body.ChapterIndex >= len(book.Chapters) {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	if body.ScrollRatio < 0 {
		body.ScrollRatio = 0
	}
	if body.ScrollRatio > 1 {
		body.ScrollRatio = 1
	}
	err := s.store.SaveProgress(book.Key, store.Progress{
		Title:        book.Title,
		Author:       book.Author,
		ChapterIndex: body.ChapterIndex,
		ScrollRatio:  body.ScrollRatio,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "1"})
}

func (s *Server) handleAddBookmark(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		ChapterIndex int     `json:"chapterIndex"`
		ScrollRatio  float64 `json:"scrollRatio"`
		Title        string  `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ChapterIndex < 0 || body.ChapterIndex >= len(book.Chapters) {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	chapterTitle := book.Chapters[body.ChapterIndex].Title
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = chapterTitle
	}
	if title == "" {
		title = fmt.Sprintf("Глава %d", body.ChapterIndex+1)
	}
	mark, err := s.store.AddBookmark(store.Bookmark{
		BookKey:      book.Key,
		Title:        title,
		ChapterIndex: body.ChapterIndex,
		ChapterTitle: chapterTitle,
		ScrollRatio:  body.ScrollRatio,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"bookmark":  mark,
		"bookmarks": s.store.Bookmarks(book.Key),
	})
}

func (s *Server) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveBookmark(id); err != nil {
		http.Error(w, "закладка не найдена", http.StatusNotFound)
		return
	}
	book := s.current()
	marks := []store.Bookmark{}
	if book != nil {
		marks = s.store.Bookmarks(book.Key)
	}
	writeJSON(w, map[string]any{"bookmarks": marks, "undo": s.store.UndoLog()})
}

func (s *Server) handleAddHighlight(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		ChapterIndex int    `json:"chapterIndex"`
		Start        int    `json:"start"`
		End          int    `json:"end"`
		Color        string `json:"color"`
		Text         string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ChapterIndex < 0 || body.ChapterIndex >= len(book.Chapters) {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	if body.End-body.Start > 20000 {
		http.Error(w, "слишком длинный фрагмент", http.StatusBadRequest)
		return
	}
	hl, err := s.store.AddHighlight(store.Highlight{
		BookKey:      book.Key,
		ChapterIndex: body.ChapterIndex,
		Start:        body.Start,
		End:          body.End,
		Color:        body.Color,
		Text:         body.Text,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"highlight":  hl,
		"highlights": s.store.Highlights(book.Key),
	})
}

func (s *Server) handleUpdateHighlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	hl, err := s.store.UpdateHighlight(body.ID, body.Color)
	if err != nil {
		http.Error(w, "выделение не найдено", http.StatusNotFound)
		return
	}
	book := s.current()
	list := []store.Highlight{}
	if book != nil {
		list = s.store.Highlights(book.Key)
	}
	writeJSON(w, map[string]any{
		"highlight":  hl,
		"highlights": list,
	})
}

func (s *Server) handleDeleteHighlight(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveHighlight(id); err != nil {
		http.Error(w, "выделение не найдено", http.StatusNotFound)
		return
	}
	book := s.current()
	list := []store.Highlight{}
	if book != nil {
		list = s.store.Highlights(book.Key)
	}
	writeJSON(w, map[string]any{"highlights": list})
}

func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		ChapterIndex int    `json:"chapterIndex"`
		Start        int    `json:"start"`
		End          int    `json:"end"`
		Color        string `json:"color"`
		Text         string `json:"text"`
		Body         string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ChapterIndex < 0 || body.ChapterIndex >= len(book.Chapters) {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	if body.End-body.Start > 20000 {
		http.Error(w, "слишком длинный фрагмент", http.StatusBadRequest)
		return
	}
	note, err := s.store.AddNote(store.Note{
		BookKey:      book.Key,
		ChapterIndex: body.ChapterIndex,
		Start:        body.Start,
		End:          body.End,
		Color:        body.Color,
		Text:         body.Text,
		Body:         body.Body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"note":  note,
		"notes": s.store.Notes(book.Key),
	})
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Color string `json:"color"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	note, err := s.store.UpdateNote(body.ID, body.Color, body.Body)
	if err != nil {
		http.Error(w, "заметка не найдена", http.StatusNotFound)
		return
	}
	book := s.current()
	list := []store.Note{}
	if book != nil {
		list = s.store.Notes(book.Key)
	}
	writeJSON(w, map[string]any{
		"note":  note,
		"notes": list,
	})
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveNote(id); err != nil {
		http.Error(w, "заметка не найдена", http.StatusNotFound)
		return
	}
	book := s.current()
	list := []store.Note{}
	if book != nil {
		list = s.store.Notes(book.Key)
	}
	writeJSON(w, map[string]any{"notes": list, "undo": s.store.UndoLog()})
}

func (s *Server) handleSaveTOCFold(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		Collapsed []string `json:"collapsed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.store.SetTOCFold(book.Key, body.Collapsed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"tocFold": s.store.TOCFold(book.Key)})
}

func (s *Server) handleSetChapterRead(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	var body struct {
		ChapterIndex int    `json:"chapterIndex"`
		TOCKey       string `json:"tocKey"`
		Read         bool   `json:"read"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.TOCKey != "" {
		if !epub.ValidTOCKey(body.TOCKey) {
			http.Error(w, "нет такого пункта оглавления", http.StatusBadRequest)
			return
		}
		item, ok := epub.LookupTOC(book.TOC, body.TOCKey)
		if !ok {
			http.Error(w, "нет такого пункта оглавления", http.StatusBadRequest)
			return
		}
		same := epub.TOCKeysForChapter(book.TOC, item.ChapterIndex)
		chapters, toc, err := s.store.SetTOCRead(book.Key, body.TOCKey, body.Read, item.ChapterIndex, same)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"readChapters": chapters, "readTOC": toc})
		return
	}
	if body.ChapterIndex < 0 || body.ChapterIndex >= len(book.Chapters) {
		http.Error(w, "bad chapter", http.StatusBadRequest)
		return
	}
	drop := epub.TOCKeysForChapter(book.TOC, body.ChapterIndex)
	list, err := s.store.SetChapterRead(book.Key, body.ChapterIndex, body.Read, drop...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"readChapters": list, "readTOC": s.store.ReadTOC(book.Key)})
}

func (s *Server) handleSaveUI(w http.ResponseWriter, r *http.Request) {
	var ui store.UI
	if err := json.NewDecoder(r.Body).Decode(&ui); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.store.SetUI(ui); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.store.UI())
}

func (s *Server) handleWelcomeBackground(w http.ResponseWriter, r *http.Request) {
	filePath, err := s.store.WelcomeBackgroundFile()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000")
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleSaveWelcomeBackground(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxWelcomeBackgroundBytes+1024)
	if err := r.ParseMultipartForm(store.MaxWelcomeBackgroundBytes); err != nil {
		http.Error(w, "файл слишком большой или повреждён", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "нужна картинка", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, store.MaxWelcomeBackgroundBytes+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.SaveWelcomeBackground(data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, s.store.UI())
}

func (s *Server) handleDeleteWelcomeBackground(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearWelcomeBackground(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.store.UI())
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "file too large or invalid", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := path.Base(strings.ReplaceAll(hdr.Filename, "\\", "/"))
	book, err := open.OpenBytes(name, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	book.Key = s.store.BindKey(book.Key, book.Title)
	if saved, err := s.store.SaveBookFile(book.Key, name, data); err == nil {
		book.Path = saved
	}
	s.recordSession(-1, -1)
	s.setBook(book)
	_ = s.remember(book)
	s.handleState(w, r)
}

func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	book, err := demoBook()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.recordSession(-1, -1)
	s.setBook(book)
	_ = s.remember(book)
	s.handleState(w, r)
}

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request) {
	book, err := guideBook()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.recordSession(-1, -1)
	s.setBook(book)
	_ = s.remember(book)
	s.handleState(w, r)
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	s.recordSession(-1, -1)
	s.setBook(nil)
	s.handleState(w, r)
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		http.Error(w, "название нужно", http.StatusBadRequest)
		return
	}
	s.recordSession(-1, -1)
	if _, err := s.store.CreateWorkspace(body.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.setBook(nil)
	s.handleState(w, r)
}

func (s *Server) handleRenameWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if _, err := s.store.RenameWorkspace(body.ID, body.Name); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "пространство не найдено", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleCreateList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if _, err := s.store.CreateList(body.Name, body.Key); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "книга не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleDeleteList(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteList(id); err != nil {
		http.Error(w, "список не найден", http.StatusNotFound)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleAddToList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" || strings.TrimSpace(body.Key) == "" {
		http.Error(w, "id and key required", http.StatusBadRequest)
		return
	}
	if err := s.store.AddToList(body.ID, body.Key); err != nil {
		http.Error(w, "не удалось добавить в список", http.StatusNotFound)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleRemoveFromList(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	key := r.URL.Query().Get("key")
	if id == "" || key == "" {
		http.Error(w, "id and key required", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveFromList(id, key); err != nil {
		http.Error(w, "книга не в списке", http.StatusNotFound)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleReorderWorkspaces(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.store.ReorderWorkspaces(body.IDs); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "пространство не найдено", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handlePinWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		Pinned bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.SetWorkspacePinned(body.ID, body.Pinned); err != nil {
		http.Error(w, "пространство не найдено", http.StatusNotFound)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleSwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	s.recordSession(-1, -1)
	if err := s.store.SwitchWorkspace(body.ID); err != nil {
		http.Error(w, "пространство не найдено", http.StatusNotFound)
		return
	}
	s.setBook(nil)
	s.handleState(w, r)
}

func (s *Server) handleMoveBook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key  string `json:"key"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Key) == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	destID := strings.TrimSpace(body.ID)
	if destID == "" {
		if strings.TrimSpace(body.Name) == "" {
			http.Error(w, "пространство нужно", http.StatusBadRequest)
			return
		}
		created, err := s.store.AddWorkspace(body.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		destID = created.ID
	}
	if book := s.current(); book != nil && book.Key == body.Key {
		s.recordSession(-1, -1)
		s.setBook(nil)
	}
	if err := s.store.MoveBook(body.Key, destID); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "не удалось перенести книгу", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleLibraryOpen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	entry, ok := s.store.Entry(body.Key)
	if !ok {
		http.Error(w, "нет в библиотеке", http.StatusNotFound)
		return
	}
	book, err := openLibrary(entry)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.recordSession(-1, -1)
	s.setBook(book)
	_ = s.remember(book)
	s.handleState(w, r)
}

func (s *Server) handleLibraryDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.book != nil && s.book.Key == key {
		old := s.book
		s.book = nil
		s.mu.Unlock()
		_ = old.Close()
	} else {
		s.mu.Unlock()
	}
	if err := s.store.Remove(key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	tmp, err := os.CreateTemp("", "boo-export-*.zip")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := s.store.Export(tmp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	info, err := tmp.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := store.ExportFileName()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, tmp)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxTransferBytes)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "file too large or invalid", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if err := s.store.Import(file); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.dropAllDicts()
	s.setBook(nil)
	s.handleState(w, r)
}

func (s *Server) dropAllDicts() {
	s.dictMu.Lock()
	s.dicts = map[string]*dict.Index{}
	s.dictMu.Unlock()
}

func (s *Server) handleLibrarySearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	hits := s.store.SearchLibrary(q)
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		e := h.Entry
		out = append(out, map[string]any{
			"key":      e.Key,
			"title":    e.Title,
			"author":   e.Author,
			"format":   e.Format,
			"coverUrl": libraryCoverURL(e),
			"finished": e.Finished,
			"canOpen":  bundledBook(e) || e.Path != "",
			"workspace": map[string]any{
				"id":   h.WorkspaceID,
				"name": h.WorkspaceName,
			},
			"lists": h.Lists,
		})
	}
	writeJSON(w, map[string]any{"query": q, "hits": out})
}

func (s *Server) handleLibraryCover(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("n")
	if name == "" {
		key := r.URL.Query().Get("key")
		entry, ok := s.store.EntryAnywhere(key)
		if !ok || entry.Cover == "" {
			http.NotFound(w, r)
			return
		}
		name = entry.Cover
	}
	path, err := s.store.CoverFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000")
	http.ServeFile(w, r, path)
}

func libraryCoverURL(e store.Entry) string {
	if e.Cover == "" {
		return ""
	}
	return "/api/library/cover?n=" + url.QueryEscape(e.Cover)
}

func (s *Server) repairSharedCovers() {
	if s.store == nil {
		return
	}
	for _, ref := range s.store.ConflictingCoverEntries() {
		book, err := openLibrary(ref.Entry)
		if err != nil {
			continue
		}
		if book.CoverHref == "" {
			_ = book.Close()
			continue
		}
		data, mime, err := book.Resource(book.CoverHref)
		_ = book.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		name, err := s.store.SaveCover(ref.Entry.Key, mime, data)
		if err != nil {
			continue
		}
		_ = s.store.SetCover(ref.WorkspaceID, ref.Entry.Key, name)
	}
}

func (s *Server) handleResource(w http.ResponseWriter, r *http.Request) {
	book := s.current()
	if book == nil {
		http.NotFound(w, r)
		return
	}
	name := r.URL.Query().Get("p")
	if name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	data, mime, err := book.Resource(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(mime, "image/") && !strings.HasPrefix(mime, "font/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}

func (s *Server) remember(book *epub.Book) error {
	if book == nil {
		return nil
	}
	book.Key = s.store.BindKey(book.Key, book.Title)
	cover := ""
	if book.CoverHref != "" {
		if data, mime, err := book.Resource(book.CoverHref); err == nil {
			if name, err := s.store.SaveCover(book.Key, mime, data); err == nil {
				cover = name
			}
		}
	}
	p, _ := s.store.Progress(book.Key)
	return s.store.Remember(store.Entry{
		Key:          book.Key,
		Title:        book.Title,
		Author:       book.Author,
		Path:         book.Path,
		Format:       bookFormat(book),
		Cover:        cover,
		ChapterIndex: p.ChapterIndex,
		ScrollRatio:  p.ScrollRatio,
		ChapterN:     len(book.Chapters),
	})
}

func (s *Server) libraryPayload() []map[string]any {
	items := s.store.Library()
	out := make([]map[string]any, 0, len(items))
	for _, e := range items {
		percent := 0.0
		if e.ChapterN > 0 {
			percent = (float64(e.ChapterIndex) + e.ScrollRatio) / float64(e.ChapterN) * 100
			if percent > 100 {
				percent = 100
			}
		}
		out = append(out, map[string]any{
			"key":          e.Key,
			"title":        e.Title,
			"author":       e.Author,
			"format":       e.Format,
			"coverUrl":     libraryCoverURL(e),
			"chapterIndex": e.ChapterIndex,
			"chapterN":     e.ChapterN,
			"percent":      int(percent),
			"finished":     e.Finished,
			"description":  e.Description,
			"journal":      e.Journal,
			"canOpen":      bundledBook(e) || e.Path != "",
		})
	}
	return out
}

func (s *Server) current() *epub.Book {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.book
}

func (s *Server) setBook(book *epub.Book) {
	s.mu.Lock()
	old := s.book
	s.book = book
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
}

func bookFormat(book *epub.Book) string {
	if book.Identifier == epub.SampleIdentifier || book.Key == "id:"+epub.SampleIdentifier {
		return "demo"
	}
	if book.Identifier == epub.GuideIdentifier || book.Key == "id:"+epub.GuideIdentifier {
		return "guide"
	}
	if book.Format != "" {
		return book.Format
	}
	return "epub"
}

func demoBook() (*epub.Book, error) {
	data, err := epub.Sample()
	if err != nil {
		return nil, err
	}
	return epub.OpenBytes("demo.epub", data)
}

func guideBook() (*epub.Book, error) {
	return epub.Guide(docs.UserGuide)
}

func bundledBook(e store.Entry) bool {
	return store.BundledFormat(e.Format) ||
		e.Key == "id:"+epub.SampleIdentifier ||
		e.Key == "id:"+epub.GuideIdentifier
}

func OpenLast(st *store.Store) *epub.Book {
	if st == nil {
		return nil
	}
	for _, e := range st.Library() {
		book, err := openLibrary(e)
		if err != nil {
			continue
		}
		return book
	}
	return nil
}

func openLibrary(e store.Entry) (*epub.Book, error) {
	if e.Format == "demo" || e.Key == "id:"+epub.SampleIdentifier {
		return demoBook()
	}
	if e.Format == "guide" || e.Key == "id:"+epub.GuideIdentifier {
		return guideBook()
	}
	if e.Path == "" {
		return nil, fmt.Errorf("файл больше недоступен")
	}
	if _, err := os.Stat(e.Path); err != nil {
		return nil, err
	}
	return open.Open(e.Path)
}

func (s *Server) handleSetFinished(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key      string `json:"key"`
		Finished bool   `json:"finished"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(body.Key)
	if key == "" {
		if book := s.current(); book != nil {
			key = book.Key
		}
	}
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	if err := s.store.SetFinished(key, body.Finished); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "книга не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleState(w, r)
}

func (s *Server) handleSetBookMeta(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Journal     string `json:"journal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(body.Key)
	if key == "" {
		if book := s.current(); book != nil {
			key = book.Key
		}
	}
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	if err := s.store.SetBookMeta(key, body.Description, body.Journal); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "книга не найдена", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleState(w, r)
}

func (s *Server) bookPayload(book *epub.Book) any {
	if book == nil {
		return nil
	}
	chapters := make([]map[string]any, len(book.Chapters))
	for i, ch := range book.Chapters {
		chapters[i] = map[string]any{
			"index": i,
			"id":    ch.ID,
			"href":  ch.Href,
			"title": ch.Title,
		}
	}
	cover := ""
	if book.CoverHref != "" {
		cover = "/res?p=" + url.QueryEscape(book.CoverHref)
	}
	finished := false
	description := ""
	journal := ""
	dictionaryID := ""
	if e, ok := s.store.Entry(book.Key); ok {
		finished = e.Finished
		description = e.Description
		journal = e.Journal
		dictionaryID = e.DictionaryID
	}
	return map[string]any{
		"key":          book.Key,
		"title":        book.Title,
		"author":       book.Author,
		"language":     book.Language,
		"format":       bookFormat(book),
		"coverUrl":     cover,
		"chapters":     chapters,
		"toc":          book.TOC,
		"chapterN":     len(book.Chapters),
		"finished":     finished,
		"description":  description,
		"journal":      journal,
		"dictionaryId": dictionaryID,
	}
}

func (s *Server) handleDictionaries(w http.ResponseWriter, r *http.Request) {
	s.writeDictionaries(w)
}

func (s *Server) writeDictionaries(w http.ResponseWriter) {
	id := ""
	if book := s.current(); book != nil {
		id = s.store.BookDictionary(book.Key)
	}
	writeJSON(w, map[string]any{
		"dictionaries": s.store.Dictionaries(),
		"dictionaryId": id,
	})
}

func (s *Server) handleAddDictionary(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "file too large or invalid", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := path.Base(strings.ReplaceAll(hdr.Filename, "\\", "/"))
	idx, err := dict.Parse(name, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ext := strings.ToLower(path.Ext(name))
	if ext == "" {
		ext = ".txt"
	}
	meta, err := s.store.AddDictionary(store.Dictionary{
		Name:      idx.Name,
		FromLang:  idx.FromLang,
		ToLang:    idx.ToLang,
		FileName:  name,
		Ext:       ext,
		WordCount: idx.Len(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.SaveDictionaryFile(meta, data); err != nil {
		_ = s.store.RemoveDictionary(meta.ID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.cacheDict(meta.ID, idx)
	if book := s.current(); book != nil {
		if err := s.store.SetBookDictionary(book.Key, meta.ID); err != nil && err != os.ErrNotExist {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.writeDictionaries(w)
}

func (s *Server) handleDeleteDictionary(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveDictionary(id); err != nil {
		http.Error(w, "словарь не найден", http.StatusNotFound)
		return
	}
	s.dropDict(id)
	s.writeDictionaries(w)
}

func (s *Server) handleSetBookDictionary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(body.Key)
	if key == "" {
		if book := s.current(); book != nil {
			key = book.Key
		}
	}
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	if err := s.store.SetBookDictionary(key, strings.TrimSpace(body.ID)); err != nil {
		if err == os.ErrNotExist {
			http.Error(w, "не найдено", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeDictionaries(w)
}

func (s *Server) handleDictionaryLookup(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, map[string]any{"query": "", "found": false, "entries": []dict.Hit{}})
		return
	}
	if runes := []rune(q); len(runes) > 80 {
		q = string(runes[:80])
	}
	book := s.current()
	if book == nil {
		http.Error(w, "no book", http.StatusNotFound)
		return
	}
	id := s.store.BookDictionary(book.Key)
	if id == "" {
		writeJSON(w, map[string]any{"query": q, "found": false, "active": false, "entries": []dict.Hit{}})
		return
	}
	idx, err := s.loadDict(id)
	if err != nil {
		http.Error(w, "словарь недоступен", http.StatusInternalServerError)
		return
	}
	hits := idx.Lookup(q)
	if hits == nil {
		hits = []dict.Hit{}
	}
	meta, _ := s.store.Dictionary(id)
	writeJSON(w, map[string]any{
		"query":    q,
		"found":    len(hits) > 0,
		"active":   true,
		"fromLang": meta.FromLang,
		"toLang":   meta.ToLang,
		"name":     meta.Name,
		"entries":  hits,
	})
}

func (s *Server) loadDict(id string) (*dict.Index, error) {
	s.dictMu.Lock()
	if idx := s.dicts[id]; idx != nil {
		s.dictMu.Unlock()
		return idx, nil
	}
	s.dictMu.Unlock()
	path, err := s.store.DictionaryFile(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, ok := s.store.Dictionary(id)
	name := "dict.txt"
	if ok && meta.FileName != "" {
		name = meta.FileName
	}
	idx, err := dict.Parse(name, data)
	if err != nil {
		return nil, err
	}
	s.cacheDict(id, idx)
	return idx, nil
}

func (s *Server) cacheDict(id string, idx *dict.Index) {
	if id == "" || idx == nil {
		return
	}
	s.dictMu.Lock()
	s.dicts[id] = idx
	s.dictMu.Unlock()
}

func (s *Server) dropDict(id string) {
	s.dictMu.Lock()
	delete(s.dicts, id)
	s.dictMu.Unlock()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func withLocalhost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "localhost only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
