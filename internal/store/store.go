package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	dir  string
	path string
	mu   sync.Mutex
	data Data
}

type Data struct {
	Workspaces       []Workspace  `json:"workspaces"`
	CurrentWorkspace string       `json:"currentWorkspace"`
	UI               UI           `json:"ui"`
	Dictionaries     []Dictionary `json:"dictionaries,omitempty"`

	// Устаревшие поля до появления рабочих пространств.
	Books      map[string]Progress `json:"books,omitempty"`
	Library    []Entry             `json:"library,omitempty"`
	Bookmarks  []Bookmark          `json:"bookmarks,omitempty"`
	Highlights []Highlight         `json:"highlights,omitempty"`
	Notes      []Note              `json:"notes,omitempty"`
	TOCFold    map[string][]string `json:"tocFold,omitempty"`
}

type Workspace struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Books        map[string]Progress `json:"books"`
	Library      []Entry             `json:"library"`
	Lists        []ReadingList       `json:"lists"`
	Bookmarks    []Bookmark          `json:"bookmarks"`
	Highlights   []Highlight         `json:"highlights"`
	Notes        []Note              `json:"notes"`
	Todos        []Todo              `json:"todos"`
	History      []HistoryEntry      `json:"history,omitempty"`
	UndoLog      []UndoAction        `json:"undoLog,omitempty"`
	TOCFold      map[string][]string `json:"tocFold"`
	ReadChapters map[string][]int    `json:"readChapters,omitempty"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

const (
	HistorySession = "session"
	HistoryNote    = "note"

	UndoDeleteBook     = "delete_book"
	UndoDeleteBookmark = "delete_bookmark"
	UndoDeleteNote     = "delete_note"
)

type HistoryEntry struct {
	ID           string    `json:"id"`
	BookKey      string    `json:"bookKey"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Kind         string    `json:"kind"`
	Text         string    `json:"text,omitempty"`
	ChapterIndex int       `json:"chapterIndex"`
	ChapterTitle string    `json:"chapterTitle,omitempty"`
	ScrollRatio  float64   `json:"scrollRatio"`
	CreatedAt    time.Time `json:"createdAt"`
}

type UndoAction struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Label     string        `json:"label"`
	Detail    string        `json:"detail,omitempty"`
	BookKey   string        `json:"bookKey,omitempty"`
	BookTitle string        `json:"bookTitle,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	Book      *BookSnapshot `json:"book,omitempty"`
	Bookmark  *Bookmark     `json:"bookmark,omitempty"`
	Note      *Note         `json:"note,omitempty"`
}

type BookSnapshot struct {
	Entry        Entry          `json:"entry"`
	Progress     *Progress      `json:"progress,omitempty"`
	Bookmarks    []Bookmark     `json:"bookmarks,omitempty"`
	Highlights   []Highlight    `json:"highlights,omitempty"`
	Notes        []Note         `json:"notes,omitempty"`
	Todos        []Todo         `json:"todos,omitempty"`
	History      []HistoryEntry `json:"history,omitempty"`
	TOCFold      []string       `json:"tocFold,omitempty"`
	ReadChapters []int          `json:"readChapters,omitempty"`
	ListIDs      []string       `json:"listIds,omitempty"`
	TrashRel     string         `json:"trashRel,omitempty"`
	FileName     string         `json:"fileName,omitempty"`
	CoverName    string         `json:"coverName,omitempty"`
}

type UndoInfo struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Label     string    `json:"label"`
	Detail    string    `json:"detail,omitempty"`
	BookKey   string    `json:"bookKey,omitempty"`
	BookTitle string    `json:"bookTitle,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Todo struct {
	ID        string    `json:"id"`
	BookKey   string    `json:"bookKey"`
	Text      string    `json:"text"`
	DueAt     time.Time `json:"dueAt"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ReadingList struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BookKeys  []string  `json:"bookKeys"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ReadingListInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BookCount int       `json:"bookCount"`
	BookKeys  []string  `json:"bookKeys"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WorkspaceInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BookCount int       `json:"bookCount"`
	UpdatedAt time.Time `json:"updatedAt"`
	Current   bool      `json:"current"`
}

type Note struct {
	ID           string    `json:"id"`
	BookKey      string    `json:"bookKey"`
	ChapterIndex int       `json:"chapterIndex"`
	Start        int       `json:"start"`
	End          int       `json:"end"`
	Color        string    `json:"color"`
	Text         string    `json:"text"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Highlight struct {
	ID           string    `json:"id"`
	BookKey      string    `json:"bookKey"`
	ChapterIndex int       `json:"chapterIndex"`
	Start        int       `json:"start"`
	End          int       `json:"end"`
	Color        string    `json:"color"`
	Text         string    `json:"text"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Bookmark struct {
	ID           string    `json:"id"`
	BookKey      string    `json:"bookKey"`
	Title        string    `json:"title"`
	ChapterIndex int       `json:"chapterIndex"`
	ChapterTitle string    `json:"chapterTitle"`
	ScrollRatio  float64   `json:"scrollRatio"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Progress struct {
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	ChapterIndex int       `json:"chapterIndex"`
	ScrollRatio  float64   `json:"scrollRatio"`
	Finished     bool      `json:"finished,omitempty"`
	FinishedAt   time.Time `json:"finishedAt,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Entry struct {
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Path         string    `json:"path"`
	Format       string    `json:"format"`
	Cover        string    `json:"cover,omitempty"`
	ChapterIndex int       `json:"chapterIndex"`
	ScrollRatio  float64   `json:"scrollRatio"`
	ChapterN     int       `json:"chapterN"`
	Finished     bool      `json:"finished,omitempty"`
	FinishedAt   time.Time `json:"finishedAt,omitempty"`
	Description  string    `json:"description,omitempty"`
	Journal      string    `json:"journal,omitempty"`
	DictionaryID string    `json:"dictionaryId,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func BundledFormat(format string) bool {
	return format == "demo" || format == "guide"
}

type Dictionary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	FromLang  string    `json:"fromLang,omitempty"`
	ToLang    string    `json:"toLang,omitempty"`
	FileName  string    `json:"fileName"`
	Ext       string    `json:"ext,omitempty"`
	WordCount int       `json:"wordCount"`
	CreatedAt time.Time `json:"createdAt"`
}

type UI struct {
	Theme            string  `json:"theme"`
	FontSize         int     `json:"fontSize"`
	BookFont         string  `json:"bookFont"`
	BookFontSize     int     `json:"bookFontSize"`
	UIFont           string  `json:"uiFont"`
	UIFontSize       int     `json:"uiFontSize"`
	LineHeight       float64 `json:"lineHeight"`
	MaxWidth         int     `json:"maxWidth"`
	SidebarWidth     int     `json:"sidebarWidth"`
	SidebarOpen      bool    `json:"sidebarOpen"`
	NotesWidth       int     `json:"notesWidth"`
	NotesOpen        bool    `json:"notesOpen"`
	HistoryWidth     int     `json:"historyWidth"`
	HistoryOpen      bool    `json:"historyOpen"`
	WorkspacesWidth  int     `json:"workspacesWidth"`
	WorkspacesOpen   bool    `json:"workspacesOpen"`
	ListsWidth       int     `json:"listsWidth"`
	ListsOpen        bool    `json:"listsOpen"`
	TocCollapsed     bool    `json:"tocCollapsed"`
	HideReadChapters bool    `json:"hideReadChapters"`
}

func Open() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	dir = filepath.Join(dir, "boo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		dir:  dir,
		path: filepath.Join(dir, "state.json"),
		data: Data{UI: defaultUI()},
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.ensureWorkspace()
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		s.ensureWorkspace()
		return s, nil
	}
	s.data.UI = normalizeUI(s.data.UI)
	migrated := s.migrateWorkspaces()
	s.normalizeWorkspaces()
	s.ensureWorkspace()
	s.migrateLibrary()
	if migrated {
		_ = s.save()
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func emptyWorkspace(id, name string) Workspace {
	now := time.Now()
	return Workspace{
		ID:           id,
		Name:         name,
		Books:        map[string]Progress{},
		TOCFold:      map[string][]string{},
		ReadChapters: map[string][]int{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func workspaceID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("ws|%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func listID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("list|%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) migrateWorkspaces() bool {
	if len(s.data.Workspaces) > 0 {
		s.clearLegacy()
		return false
	}
	hasLegacy := len(s.data.Library) > 0 || len(s.data.Books) > 0 || len(s.data.Bookmarks) > 0 ||
		len(s.data.Highlights) > 0 || len(s.data.Notes) > 0 || len(s.data.TOCFold) > 0
	if !hasLegacy {
		return false
	}
	ws := emptyWorkspace(workspaceID(), "Библиотека")
	ws.Books = s.data.Books
	ws.Library = s.data.Library
	ws.Bookmarks = s.data.Bookmarks
	ws.Highlights = s.data.Highlights
	ws.Notes = s.data.Notes
	ws.TOCFold = s.data.TOCFold
	s.data.Workspaces = []Workspace{ws}
	s.data.CurrentWorkspace = ws.ID
	s.clearLegacy()
	return true
}

func (s *Store) clearLegacy() {
	s.data.Books = nil
	s.data.Library = nil
	s.data.Bookmarks = nil
	s.data.Highlights = nil
	s.data.Notes = nil
	s.data.TOCFold = nil
}

func (s *Store) normalizeWorkspaces() {
	for i := range s.data.Workspaces {
		ws := &s.data.Workspaces[i]
		if ws.Books == nil {
			ws.Books = map[string]Progress{}
		}
		if ws.Bookmarks == nil {
			ws.Bookmarks = []Bookmark{}
		}
		if ws.Highlights == nil {
			ws.Highlights = []Highlight{}
		}
		if ws.Notes == nil {
			ws.Notes = []Note{}
		}
		if ws.Todos == nil {
			ws.Todos = []Todo{}
		}
		if ws.History == nil {
			ws.History = []HistoryEntry{}
		}
		if ws.UndoLog == nil {
			ws.UndoLog = []UndoAction{}
		}
		if ws.TOCFold == nil {
			ws.TOCFold = map[string][]string{}
		}
		if ws.ReadChapters == nil {
			ws.ReadChapters = map[string][]int{}
		}
		if ws.Lists == nil {
			ws.Lists = []ReadingList{}
		}
		for j := range ws.Lists {
			if ws.Lists[j].BookKeys == nil {
				ws.Lists[j].BookKeys = []string{}
			}
			if strings.TrimSpace(ws.Lists[j].Name) == "" {
				ws.Lists[j].Name = "Список"
			}
			if ws.Lists[j].ID == "" {
				ws.Lists[j].ID = listID()
			}
		}
		if strings.TrimSpace(ws.Name) == "" {
			ws.Name = "Библиотека"
		}
		if ws.ID == "" {
			ws.ID = workspaceID()
		}
	}
}

func (s *Store) ensureWorkspace() {
	if len(s.data.Workspaces) == 0 {
		ws := emptyWorkspace(workspaceID(), "Библиотека")
		s.data.Workspaces = []Workspace{ws}
		s.data.CurrentWorkspace = ws.ID
		return
	}
	if s.data.CurrentWorkspace != "" {
		for _, ws := range s.data.Workspaces {
			if ws.ID == s.data.CurrentWorkspace {
				return
			}
		}
	}
	best := 0
	for i := 1; i < len(s.data.Workspaces); i++ {
		if s.data.Workspaces[i].UpdatedAt.After(s.data.Workspaces[best].UpdatedAt) {
			best = i
		}
	}
	s.data.CurrentWorkspace = s.data.Workspaces[best].ID
}

func (s *Store) ws() *Workspace {
	s.ensureWorkspace()
	for i := range s.data.Workspaces {
		if s.data.Workspaces[i].ID == s.data.CurrentWorkspace {
			return &s.data.Workspaces[i]
		}
	}
	return &s.data.Workspaces[0]
}

func (s *Store) Workspaces() []WorkspaceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	out := make([]WorkspaceInfo, 0, len(s.data.Workspaces))
	for _, ws := range s.data.Workspaces {
		out = append(out, WorkspaceInfo{
			ID:        ws.ID,
			Name:      ws.Name,
			BookCount: len(ws.Library),
			UpdatedAt: ws.UpdatedAt,
			Current:   ws.ID == s.data.CurrentWorkspace,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Store) CurrentWorkspace() WorkspaceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	return WorkspaceInfo{
		ID:        ws.ID,
		Name:      ws.Name,
		BookCount: len(ws.Library),
		UpdatedAt: ws.UpdatedAt,
		Current:   true,
	}
}

func (s *Store) CreateWorkspace(name string) (WorkspaceInfo, error) {
	return s.addWorkspace(name, true)
}

func (s *Store) AddWorkspace(name string) (WorkspaceInfo, error) {
	return s.addWorkspace(name, false)
}

func (s *Store) addWorkspace(name string, switchTo bool) (WorkspaceInfo, error) {
	name = clipRunes(name, 80)
	if name == "" {
		return WorkspaceInfo{}, fmt.Errorf("название нужно")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := emptyWorkspace(workspaceID(), name)
	s.data.Workspaces = append(s.data.Workspaces, ws)
	if switchTo {
		s.data.CurrentWorkspace = ws.ID
	}
	if err := s.save(); err != nil {
		return WorkspaceInfo{}, err
	}
	return WorkspaceInfo{ID: ws.ID, Name: ws.Name, Current: switchTo, UpdatedAt: ws.UpdatedAt}, nil
}

func (s *Store) RenameWorkspace(id, name string) (WorkspaceInfo, error) {
	name = clipRunes(name, 80)
	if name == "" {
		return WorkspaceInfo{}, fmt.Errorf("название нужно")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkspaceInfo{}, os.ErrNotExist
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	for i := range s.data.Workspaces {
		if s.data.Workspaces[i].ID != id {
			continue
		}
		s.data.Workspaces[i].Name = name
		s.data.Workspaces[i].UpdatedAt = time.Now()
		if err := s.save(); err != nil {
			return WorkspaceInfo{}, err
		}
		ws := s.data.Workspaces[i]
		return WorkspaceInfo{
			ID:        ws.ID,
			Name:      ws.Name,
			BookCount: len(ws.Library),
			UpdatedAt: ws.UpdatedAt,
			Current:   ws.ID == s.data.CurrentWorkspace,
		}, nil
	}
	return WorkspaceInfo{}, os.ErrNotExist
}

func (s *Store) Lists() []ReadingListInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	out := make([]ReadingListInfo, 0, len(ws.Lists))
	for _, list := range ws.Lists {
		out = append(out, listInfo(list))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func listInfo(list ReadingList) ReadingListInfo {
	keys := append([]string(nil), list.BookKeys...)
	if keys == nil {
		keys = []string{}
	}
	return ReadingListInfo{
		ID:        list.ID,
		Name:      list.Name,
		BookCount: len(list.BookKeys),
		BookKeys:  keys,
		UpdatedAt: list.UpdatedAt,
	}
}

func (s *Store) CreateList(name, bookKey string) (ReadingListInfo, error) {
	name = clipRunes(name, 80)
	if name == "" {
		return ReadingListInfo{}, fmt.Errorf("название нужно")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	keys := []string{}
	if bookKey != "" {
		if !libraryHas(ws, bookKey) {
			return ReadingListInfo{}, os.ErrNotExist
		}
		keys = []string{bookKey}
	}
	now := time.Now()
	list := ReadingList{
		ID:        listID(),
		Name:      name,
		BookKeys:  keys,
		CreatedAt: now,
		UpdatedAt: now,
	}
	ws.Lists = append(ws.Lists, list)
	ws.UpdatedAt = now
	if err := s.save(); err != nil {
		return ReadingListInfo{}, err
	}
	return listInfo(list), nil
}

func (s *Store) AddToList(id, bookKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	if !libraryHas(ws, bookKey) {
		return os.ErrNotExist
	}
	for i := range ws.Lists {
		if ws.Lists[i].ID != id {
			continue
		}
		for _, key := range ws.Lists[i].BookKeys {
			if key == bookKey {
				return nil
			}
		}
		now := time.Now()
		ws.Lists[i].BookKeys = append(ws.Lists[i].BookKeys, bookKey)
		ws.Lists[i].UpdatedAt = now
		ws.UpdatedAt = now
		return s.save()
	}
	return os.ErrNotExist
}

func (s *Store) RemoveFromList(id, bookKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	for i := range ws.Lists {
		if ws.Lists[i].ID != id {
			continue
		}
		var keep []string
		found := false
		for _, key := range ws.Lists[i].BookKeys {
			if key == bookKey {
				found = true
				continue
			}
			keep = append(keep, key)
		}
		if !found {
			return os.ErrNotExist
		}
		if keep == nil {
			keep = []string{}
		}
		now := time.Now()
		ws.Lists[i].BookKeys = keep
		ws.Lists[i].UpdatedAt = now
		ws.UpdatedAt = now
		return s.save()
	}
	return os.ErrNotExist
}

func (s *Store) DeleteList(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	var keep []ReadingList
	found := false
	for _, list := range ws.Lists {
		if list.ID == id {
			found = true
			continue
		}
		keep = append(keep, list)
	}
	if !found {
		return os.ErrNotExist
	}
	ws.Lists = keep
	ws.UpdatedAt = time.Now()
	return s.save()
}

func libraryHas(ws *Workspace, key string) bool {
	for _, e := range ws.Library {
		if e.Key == key {
			return true
		}
	}
	return false
}

func (s *Store) SwitchWorkspace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	for i := range s.data.Workspaces {
		if s.data.Workspaces[i].ID != id {
			continue
		}
		s.data.CurrentWorkspace = id
		s.data.Workspaces[i].UpdatedAt = time.Now()
		return s.save()
	}
	return os.ErrNotExist
}

func (s *Store) MoveBook(key, destID string) error {
	key = strings.TrimSpace(key)
	destID = strings.TrimSpace(destID)
	if key == "" {
		return fmt.Errorf("книга не найдена")
	}
	if destID == "" {
		return fmt.Errorf("пространство не найдено")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspace()
	src := s.ws()
	if src.ID == destID {
		return fmt.Errorf("книга уже в этом пространстве")
	}
	dest := workspaceByID(s.data.Workspaces, destID)
	if dest == nil {
		return os.ErrNotExist
	}
	entry, ok := libraryEntry(src, key)
	if !ok {
		return os.ErrNotExist
	}
	if libraryHas(dest, key) {
		return fmt.Errorf("книга уже есть в этом пространстве")
	}
	now := time.Now()
	entry.UpdatedAt = now
	moveBookToWorkspace(dest, src, entry)
	stripBookFromWorkspace(src, key)
	src.UpdatedAt = now
	dest.UpdatedAt = now
	return s.save()
}

func workspaceByID(workspaces []Workspace, id string) *Workspace {
	for i := range workspaces {
		if workspaces[i].ID == id {
			return &workspaces[i]
		}
	}
	return nil
}

func libraryEntry(ws *Workspace, key string) (Entry, bool) {
	for _, e := range ws.Library {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

func moveBookToWorkspace(dest, src *Workspace, entry Entry) {
	dest.Library = append([]Entry{entry}, dest.Library...)
	if dest.Books == nil {
		dest.Books = map[string]Progress{}
	}
	if p, ok := src.Books[entry.Key]; ok {
		dest.Books[entry.Key] = p
	}
	dest.Bookmarks = append(dest.Bookmarks, bookItems(src.Bookmarks, entry.Key, func(b Bookmark) string { return b.BookKey })...)
	dest.Highlights = append(dest.Highlights, bookItems(src.Highlights, entry.Key, func(h Highlight) string { return h.BookKey })...)
	dest.Notes = append(dest.Notes, bookItems(src.Notes, entry.Key, func(n Note) string { return n.BookKey })...)
	dest.Todos = append(dest.Todos, bookItems(src.Todos, entry.Key, func(t Todo) string { return t.BookKey })...)
	dest.History = append(bookItems(src.History, entry.Key, func(h HistoryEntry) string { return h.BookKey }), dest.History...)
	if dest.TOCFold == nil {
		dest.TOCFold = map[string][]string{}
	}
	if fold := src.TOCFold[entry.Key]; len(fold) > 0 {
		dest.TOCFold[entry.Key] = append([]string(nil), fold...)
	}
	if dest.ReadChapters == nil {
		dest.ReadChapters = map[string][]int{}
	}
	if ch := src.ReadChapters[entry.Key]; len(ch) > 0 {
		dest.ReadChapters[entry.Key] = append([]int(nil), ch...)
	}
}

func stripBookFromWorkspace(ws *Workspace, key string) {
	var keep []Entry
	for _, e := range ws.Library {
		if e.Key != key {
			keep = append(keep, e)
		}
	}
	ws.Library = keep
	delete(ws.Books, key)
	ws.Bookmarks = withoutBookItems(ws.Bookmarks, key, func(b Bookmark) string { return b.BookKey })
	ws.Highlights = withoutBookItems(ws.Highlights, key, func(h Highlight) string { return h.BookKey })
	ws.Notes = withoutBookItems(ws.Notes, key, func(n Note) string { return n.BookKey })
	ws.Todos = withoutBookItems(ws.Todos, key, func(t Todo) string { return t.BookKey })
	ws.History = withoutBookItems(ws.History, key, func(h HistoryEntry) string { return h.BookKey })
	delete(ws.TOCFold, key)
	delete(ws.ReadChapters, key)
	for i := range ws.Lists {
		var keepKeys []string
		for _, k := range ws.Lists[i].BookKeys {
			if k != key {
				keepKeys = append(keepKeys, k)
			}
		}
		if keepKeys == nil {
			keepKeys = []string{}
		}
		ws.Lists[i].BookKeys = keepKeys
	}
}

func bookItems[T any](src []T, key string, keyOf func(T) string) []T {
	var out []T
	for _, item := range src {
		if keyOf(item) == key {
			out = append(out, item)
		}
	}
	return out
}

func withoutBookItems[T any](src []T, key string, keyOf func(T) string) []T {
	var out []T
	for _, item := range src {
		if keyOf(item) != key {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) migrateLibrary() {
	for i := range s.data.Workspaces {
		ws := &s.data.Workspaces[i]
		have := map[string]bool{}
		for _, e := range ws.Library {
			have[e.Key] = true
		}
		for key, p := range ws.Books {
			if have[key] {
				continue
			}
			ws.Library = append(ws.Library, Entry{
				Key:          key,
				Title:        p.Title,
				Author:       p.Author,
				ChapterIndex: p.ChapterIndex,
				ScrollRatio:  p.ScrollRatio,
				Finished:     p.Finished,
				FinishedAt:   p.FinishedAt,
				UpdatedAt:    p.UpdatedAt,
			})
		}
	}
}

func defaultUI() UI {
	return UI{
		Theme:        "dark",
		FontSize:     20,
		BookFont:     "serif",
		BookFontSize: 20,
		UIFont:       "system",
		UIFontSize:   16,
		LineHeight:   1.7,
		MaxWidth:     38,
		SidebarWidth: 280,
		SidebarOpen:  true,
		NotesWidth:      300,
		NotesOpen:       true,
		HistoryWidth:    280,
		WorkspacesWidth: 280,
		WorkspacesOpen:  true,
		ListsWidth:      280,
		ListsOpen:       true,
	}
}

var allowedFonts = map[string]bool{
	"serif":    true,
	"georgia":  true,
	"palatino": true,
	"times":    true,
	"garamond": true,
	"cambria":  true,
	"system":   true,
	"segoe":    true,
	"arial":    true,
	"verdana":  true,
	"tahoma":   true,
	"calibri":  true,
}

func normalizeFont(id, fallback string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if allowedFonts[id] {
		return id
	}
	return fallback
}

func normalizeUI(u UI) UI {
	d := defaultUI()
	switch u.Theme {
	case "light", "sepia", "dark":
	default:
		u.Theme = d.Theme
	}
	if u.BookFontSize == 0 {
		u.BookFontSize = u.FontSize
	}
	if u.BookFontSize < 14 || u.BookFontSize > 36 {
		u.BookFontSize = d.BookFontSize
	}
	u.FontSize = u.BookFontSize
	u.BookFont = normalizeFont(u.BookFont, d.BookFont)
	u.UIFont = normalizeFont(u.UIFont, d.UIFont)
	if u.UIFontSize < 12 || u.UIFontSize > 24 {
		u.UIFontSize = d.UIFontSize
	}
	if u.LineHeight < 1.3 || u.LineHeight > 2.2 {
		u.LineHeight = d.LineHeight
	}
	if u.MaxWidth < 22 || u.MaxWidth > 80 {
		u.MaxWidth = d.MaxWidth
	}
	if u.SidebarWidth < 180 || u.SidebarWidth > 560 {
		u.SidebarWidth = d.SidebarWidth
	}
	if u.NotesWidth < 220 || u.NotesWidth > 520 {
		u.NotesWidth = d.NotesWidth
	}
	if u.HistoryWidth < 220 || u.HistoryWidth > 480 {
		u.HistoryWidth = d.HistoryWidth
	}
	if u.WorkspacesWidth == 0 {
		u.WorkspacesWidth = d.WorkspacesWidth
		u.WorkspacesOpen = true
	} else if u.WorkspacesWidth < 220 || u.WorkspacesWidth > 480 {
		u.WorkspacesWidth = d.WorkspacesWidth
	}
	if u.ListsWidth == 0 {
		u.ListsWidth = d.ListsWidth
		u.ListsOpen = true
	} else if u.ListsWidth < 220 || u.ListsWidth > 480 {
		u.ListsWidth = d.ListsWidth
	}
	return u
}

func (s *Store) UI() UI {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.UI
}

func (s *Store) SetUI(u UI) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UI = normalizeUI(u)
	return s.save()
}

const maxDictionaries = 24

func dictionaryID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("dict|%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) Dictionaries() []Dictionary {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Dictionaries) == 0 {
		return []Dictionary{}
	}
	return append([]Dictionary(nil), s.data.Dictionaries...)
}

func (s *Store) Dictionary(id string) (Dictionary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dictionaryLocked(id)
}

func (s *Store) dictionaryLocked(id string) (Dictionary, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Dictionary{}, false
	}
	for _, d := range s.data.Dictionaries {
		if d.ID == id {
			return d, true
		}
	}
	return Dictionary{}, false
}

func (s *Store) AddDictionary(d Dictionary) (Dictionary, error) {
	d.Name = clipRunes(d.Name, 120)
	d.FromLang = clipRunes(d.FromLang, 8)
	d.ToLang = clipRunes(d.ToLang, 8)
	d.FileName = clipRunes(filepath.Base(d.FileName), 180)
	if d.Name == "" {
		d.Name = "Словарь"
	}
	if d.WordCount < 0 {
		d.WordCount = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Dictionaries) >= maxDictionaries {
		return Dictionary{}, fmt.Errorf("слишком много словарей")
	}
	d.ID = dictionaryID()
	d.CreatedAt = time.Now()
	s.data.Dictionaries = append(s.data.Dictionaries, d)
	if err := s.save(); err != nil {
		return Dictionary{}, err
	}
	return d, nil
}

func (s *Store) RemoveDictionary(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	var keep []Dictionary
	var removed Dictionary
	found := false
	for _, d := range s.data.Dictionaries {
		if d.ID == id {
			found = true
			removed = d
			continue
		}
		keep = append(keep, d)
	}
	if !found {
		return os.ErrNotExist
	}
	if keep == nil {
		keep = []Dictionary{}
	}
	s.data.Dictionaries = keep
	for i := range s.data.Workspaces {
		for j := range s.data.Workspaces[i].Library {
			if s.data.Workspaces[i].Library[j].DictionaryID == id {
				s.data.Workspaces[i].Library[j].DictionaryID = ""
			}
		}
	}
	if err := s.save(); err != nil {
		return err
	}
	_ = os.Remove(s.dictionaryPathLocked(removed))
	return nil
}

func (s *Store) SetBookDictionary(bookKey, dictID string) error {
	bookKey = strings.TrimSpace(bookKey)
	dictID = strings.TrimSpace(dictID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if dictID != "" {
		if _, ok := s.dictionaryLocked(dictID); !ok {
			return os.ErrNotExist
		}
	}
	ws := s.ws()
	for i := range ws.Library {
		if ws.Library[i].Key != bookKey {
			continue
		}
		ws.Library[i].DictionaryID = dictID
		ws.Library[i].UpdatedAt = time.Now()
		ws.UpdatedAt = ws.Library[i].UpdatedAt
		return s.save()
	}
	return os.ErrNotExist
}

func (s *Store) BookDictionary(bookKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.ws().Library {
		if e.Key == bookKey {
			return e.DictionaryID
		}
	}
	return ""
}

func (s *Store) SaveDictionaryFile(d Dictionary, data []byte) (string, error) {
	dir := filepath.Join(s.dir, "dictionaries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := s.dictionaryPath(d)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) DictionaryFile(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dictionaryLocked(id)
	if !ok {
		return "", os.ErrNotExist
	}
	path := s.dictionaryPathLocked(d)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) dictionaryPath(d Dictionary) string {
	return s.dictionaryPathLocked(d)
}

func (s *Store) dictionaryPathLocked(d Dictionary) string {
	ext := strings.ToLower(d.Ext)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(d.FileName))
	}
	if ext == "" {
		ext = ".txt"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.Join(s.dir, "dictionaries", fileID(d.ID)+ext)
}

func (s *Store) Progress(key string) (Progress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.ws().Books[key]
	return p, ok
}

func (s *Store) SaveProgress(key string, p Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	if old, ok := ws.Books[key]; ok {
		p.Finished = old.Finished
		p.FinishedAt = old.FinishedAt
	}
	p.UpdatedAt = time.Now()
	ws.Books[key] = p
	ws.UpdatedAt = p.UpdatedAt
	for i := range ws.Library {
		if ws.Library[i].Key == key {
			ws.Library[i].Title = p.Title
			ws.Library[i].Author = p.Author
			ws.Library[i].ChapterIndex = p.ChapterIndex
			ws.Library[i].ScrollRatio = p.ScrollRatio
			ws.Library[i].UpdatedAt = p.UpdatedAt
			break
		}
	}
	return s.save()
}

func (s *Store) SetFinished(key string, finished bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	now := time.Now()
	found := false
	var finishedAt time.Time
	if finished {
		finishedAt = now
	}
	for i := range ws.Library {
		if ws.Library[i].Key != key {
			continue
		}
		found = true
		ws.Library[i].Finished = finished
		ws.Library[i].FinishedAt = finishedAt
		ws.Library[i].UpdatedAt = now
		break
	}
	if !found {
		return os.ErrNotExist
	}
	if p, ok := ws.Books[key]; ok {
		p.Finished = finished
		p.FinishedAt = finishedAt
		p.UpdatedAt = now
		ws.Books[key] = p
	}
	ws.UpdatedAt = now
	return s.save()
}

const (
	maxBookDescription = 2000
	maxBookJournal     = 32000
)

func (s *Store) SetBookMeta(key, description, journal string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	now := time.Now()
	description = clipRunes(description, maxBookDescription)
	journal = clipRunes(journal, maxBookJournal)
	for i := range ws.Library {
		if ws.Library[i].Key != key {
			continue
		}
		ws.Library[i].Description = description
		ws.Library[i].Journal = journal
		ws.Library[i].UpdatedAt = now
		ws.UpdatedAt = now
		return s.save()
	}
	return os.ErrNotExist
}

func (s *Store) Remember(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	e.UpdatedAt = time.Now()
	for _, old := range ws.Library {
		if old.Key != e.Key {
			continue
		}
		if e.Path == "" {
			e.Path = old.Path
		}
		if e.Cover == "" {
			e.Cover = old.Cover
		}
		if e.ChapterN == 0 {
			e.ChapterN = old.ChapterN
		}
		e.Finished = old.Finished
		e.FinishedAt = old.FinishedAt
		e.Description = old.Description
		e.Journal = old.Journal
		e.DictionaryID = old.DictionaryID
	}
	if e.Path == "" || e.Cover == "" {
		path, cover := s.sharedFiles(e.Key, ws.ID)
		if e.Path == "" {
			e.Path = path
		}
		if e.Cover == "" {
			e.Cover = cover
		}
	}
	out := []Entry{e}
	for _, old := range ws.Library {
		if old.Key == e.Key {
			continue
		}
		out = append(out, old)
	}
	ws.Library = out
	ws.Books[e.Key] = Progress{
		Title:        e.Title,
		Author:       e.Author,
		ChapterIndex: e.ChapterIndex,
		ScrollRatio:  e.ScrollRatio,
		Finished:     e.Finished,
		FinishedAt:   e.FinishedAt,
		UpdatedAt:    e.UpdatedAt,
	}
	ws.UpdatedAt = e.UpdatedAt
	return s.save()
}

func (s *Store) sharedFiles(key, exceptID string) (path, cover string) {
	for _, ws := range s.data.Workspaces {
		if ws.ID == exceptID {
			continue
		}
		for _, e := range ws.Library {
			if e.Key != key {
				continue
			}
			if path == "" {
				path = e.Path
			}
			if cover == "" {
				cover = e.Cover
			}
			if path != "" && cover != "" {
				return path, cover
			}
		}
	}
	return path, cover
}

func (s *Store) Library() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]Entry(nil), s.ws().Library...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *Store) Entry(key string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.ws().Library {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

func (s *Store) bookUsedElsewhere(key, exceptID string) bool {
	for _, ws := range s.data.Workspaces {
		if ws.ID == exceptID {
			continue
		}
		for _, e := range ws.Library {
			if e.Key == key {
				return true
			}
		}
	}
	return false
}

func (s *Store) Remove(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	shared := s.bookUsedElsewhere(key, ws.ID)
	var entry Entry
	found := false
	var keep []Entry
	for _, e := range ws.Library {
		if e.Key != key {
			keep = append(keep, e)
			continue
		}
		entry = e
		found = true
	}
	var action UndoAction
	if found {
		action = s.snapshotBookLocked(key, entry, shared)
	}
	ws.Library = keep
	delete(ws.Books, key)
	var marks []Bookmark
	for _, b := range ws.Bookmarks {
		if b.BookKey != key {
			marks = append(marks, b)
		}
	}
	ws.Bookmarks = marks
	var hls []Highlight
	for _, h := range ws.Highlights {
		if h.BookKey != key {
			hls = append(hls, h)
		}
	}
	ws.Highlights = hls
	var notes []Note
	for _, n := range ws.Notes {
		if n.BookKey != key {
			notes = append(notes, n)
		}
	}
	ws.Notes = notes
	var todos []Todo
	for _, t := range ws.Todos {
		if t.BookKey != key {
			todos = append(todos, t)
		}
	}
	ws.Todos = todos
	var history []HistoryEntry
	for _, h := range ws.History {
		if h.BookKey != key {
			history = append(history, h)
		}
	}
	ws.History = history
	delete(ws.TOCFold, key)
	delete(ws.ReadChapters, key)
	for i := range ws.Lists {
		var keepKeys []string
		for _, k := range ws.Lists[i].BookKeys {
			if k != key {
				keepKeys = append(keepKeys, k)
			}
		}
		ws.Lists[i].BookKeys = keepKeys
	}
	ws.UpdatedAt = time.Now()
	if found {
		s.pushUndoLocked(action)
	}
	return s.save()
}

func (s *Store) TOCFold(bookKey string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ws().TOCFold[bookKey]...)
}

func (s *Store) SetTOCFold(bookKey string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	if ws.TOCFold == nil {
		ws.TOCFold = map[string][]string{}
	}
	seen := map[string]bool{}
	var clean []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		delete(ws.TOCFold, bookKey)
	} else {
		ws.TOCFold[bookKey] = clean
	}
	return s.save()
}

func (s *Store) ReadChapters(bookKey string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.ws().ReadChapters[bookKey]
	if len(src) == 0 {
		return []int{}
	}
	return append([]int(nil), src...)
}

func (s *Store) SetChapterRead(bookKey string, index int, read bool) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 {
		return nil, fmt.Errorf("bad chapter")
	}
	ws := s.ws()
	if ws.ReadChapters == nil {
		ws.ReadChapters = map[string][]int{}
	}
	seen := map[int]bool{}
	var clean []int
	for _, i := range ws.ReadChapters[bookKey] {
		if i < 0 || seen[i] {
			continue
		}
		seen[i] = true
		clean = append(clean, i)
	}
	if read {
		if !seen[index] {
			clean = append(clean, index)
		}
	} else {
		keep := clean[:0]
		for _, i := range clean {
			if i != index {
				keep = append(keep, i)
			}
		}
		clean = keep
	}
	sort.Ints(clean)
	if len(clean) == 0 {
		delete(ws.ReadChapters, bookKey)
	} else {
		ws.ReadChapters[bookKey] = clean
	}
	ws.UpdatedAt = time.Now()
	if err := s.save(); err != nil {
		return nil, err
	}
	if clean == nil {
		clean = []int{}
	}
	return append([]int(nil), clean...), nil
}

func (s *Store) Bookmarks(bookKey string) []Bookmark {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Bookmark
	for _, b := range s.ws().Bookmarks {
		if b.BookKey == bookKey {
			out = append(out, b)
		}
	}
	return out
}

func (s *Store) AddBookmark(b Bookmark) (Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ScrollRatio < 0 {
		b.ScrollRatio = 0
	}
	if b.ScrollRatio > 1 {
		b.ScrollRatio = 1
	}
	b.ID = bookmarkID(b)
	b.CreatedAt = time.Now()
	if strings.TrimSpace(b.Title) == "" {
		b.Title = b.ChapterTitle
	}
	ws := s.ws()
	for _, old := range ws.Bookmarks {
		if old.BookKey == b.BookKey && old.ChapterIndex == b.ChapterIndex && abs(old.ScrollRatio-b.ScrollRatio) < 0.02 {
			return old, nil
		}
	}
	ws.Bookmarks = append([]Bookmark{b}, ws.Bookmarks...)
	return b, s.save()
}

func (s *Store) RemoveBookmark(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	var keep []Bookmark
	var removed Bookmark
	found := false
	for _, b := range ws.Bookmarks {
		if b.ID == id {
			found = true
			removed = b
			continue
		}
		keep = append(keep, b)
	}
	if !found {
		return os.ErrNotExist
	}
	ws.Bookmarks = keep
	title := strings.TrimSpace(removed.Title)
	if title == "" {
		title = strings.TrimSpace(removed.ChapterTitle)
	}
	if title == "" {
		title = "Закладка"
	}
	s.pushUndoLocked(UndoAction{
		Kind:      UndoDeleteBookmark,
		Label:     "Удалена закладка",
		Detail:    title,
		BookKey:   removed.BookKey,
		BookTitle: s.bookTitleLocked(removed.BookKey),
		Bookmark:  &removed,
	})
	return s.save()
}

var highlightColors = map[string]bool{
	"yellow": true,
	"green":  true,
	"blue":   true,
	"pink":   true,
	"orange": true,
}

func (s *Store) Highlights(bookKey string) []Highlight {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Highlight
	for _, h := range s.ws().Highlights {
		if h.BookKey == bookKey {
			out = append(out, h)
		}
	}
	return out
}

func NormalizeColor(color string) string {
	color = strings.ToLower(strings.TrimSpace(color))
	if highlightColors[color] {
		return color
	}
	return "yellow"
}

func (s *Store) AddHighlight(h Highlight) (Highlight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h.End <= h.Start {
		return Highlight{}, fmt.Errorf("пустой фрагмент")
	}
	h.Color = NormalizeColor(h.Color)
	h.Text = strings.TrimSpace(h.Text)
	if runes := []rune(h.Text); len(runes) > 240 {
		h.Text = string(runes[:240])
	}
	h.ID = highlightID(h)
	h.CreatedAt = time.Now()
	ws := s.ws()
	var keep []Highlight
	for _, old := range ws.Highlights {
		if old.BookKey == h.BookKey && old.ChapterIndex == h.ChapterIndex && old.Start < h.End && h.Start < old.End {
			continue
		}
		keep = append(keep, old)
	}
	ws.Highlights = append([]Highlight{h}, keep...)
	return h, s.save()
}

func (s *Store) UpdateHighlight(id, color string) (Highlight, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	color = NormalizeColor(color)
	ws := s.ws()
	for i, h := range ws.Highlights {
		if h.ID != id {
			continue
		}
		h.Color = color
		ws.Highlights[i] = h
		return h, s.save()
	}
	return Highlight{}, os.ErrNotExist
}

func (s *Store) RemoveHighlight(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	var keep []Highlight
	found := false
	for _, h := range ws.Highlights {
		if h.ID == id {
			found = true
			continue
		}
		keep = append(keep, h)
	}
	if !found {
		return os.ErrNotExist
	}
	ws.Highlights = keep
	return s.save()
}

func (s *Store) Notes(bookKey string) []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Note
	for _, n := range s.ws().Notes {
		if n.BookKey == bookKey {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ChapterIndex != out[j].ChapterIndex {
			return out[i].ChapterIndex < out[j].ChapterIndex
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func clipRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if runes := []rune(s); len(runes) > max {
		return string(runes[:max])
	}
	return s
}

func (s *Store) AddNote(n Note) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.End <= n.Start {
		return Note{}, fmt.Errorf("пустой фрагмент")
	}
	n.Color = NormalizeColor(n.Color)
	n.Text = clipRunes(n.Text, 240)
	n.Body = clipRunes(n.Body, 4000)
	now := time.Now()
	n.ID = noteID(n)
	n.CreatedAt = now
	n.UpdatedAt = now
	ws := s.ws()
	ws.Notes = append([]Note{n}, ws.Notes...)
	return n, s.save()
}

func (s *Store) UpdateNote(id, color, body string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	for i, n := range ws.Notes {
		if n.ID != id {
			continue
		}
		if strings.TrimSpace(color) != "" {
			n.Color = NormalizeColor(color)
		}
		n.Body = clipRunes(body, 4000)
		n.UpdatedAt = time.Now()
		ws.Notes[i] = n
		return n, s.save()
	}
	return Note{}, os.ErrNotExist
}

const maxTodoText = 400

func ParseDueAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("нужно время исполнения")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("неверное время исполнения")
}

func (s *Store) Todos(bookKey string) []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Todo
	for _, t := range s.ws().Todos {
		if t.BookKey == bookKey {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Done != out[j].Done {
			return !out[i].Done
		}
		if !out[i].DueAt.Equal(out[j].DueAt) {
			return out[i].DueAt.Before(out[j].DueAt)
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if out == nil {
		out = []Todo{}
	}
	return out
}

func (s *Store) AddTodo(t Todo) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.BookKey = strings.TrimSpace(t.BookKey)
	t.Text = clipRunes(t.Text, maxTodoText)
	if t.BookKey == "" {
		return Todo{}, fmt.Errorf("книга нужна")
	}
	if t.Text == "" {
		return Todo{}, fmt.Errorf("нужен текст")
	}
	if t.DueAt.IsZero() {
		return Todo{}, fmt.Errorf("нужно время исполнения")
	}
	ws := s.ws()
	if !libraryHas(ws, t.BookKey) {
		return Todo{}, os.ErrNotExist
	}
	now := time.Now()
	t.ID = todoID(t)
	t.Done = false
	t.CreatedAt = now
	t.UpdatedAt = now
	ws.Todos = append(ws.Todos, t)
	ws.UpdatedAt = now
	return t, s.save()
}

func (s *Store) UpdateTodo(id, text string, dueAt time.Time, done bool) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	if id == "" {
		return Todo{}, os.ErrNotExist
	}
	ws := s.ws()
	for i, t := range ws.Todos {
		if t.ID != id {
			continue
		}
		if trimmed := clipRunes(text, maxTodoText); trimmed != "" {
			t.Text = trimmed
		}
		if !dueAt.IsZero() {
			t.DueAt = dueAt
		}
		t.Done = done
		t.UpdatedAt = time.Now()
		ws.Todos[i] = t
		ws.UpdatedAt = t.UpdatedAt
		return t, s.save()
	}
	return Todo{}, os.ErrNotExist
}

const (
	historyRecent      = 10
	maxHistoryKeep     = 200
	maxHistoryText     = 400
	sessionDedupWindow = 2 * time.Minute
)

func historyID(e HistoryEntry) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("hist|%s|%s|%d", e.BookKey, e.Kind, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) History(limit int) []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.ws().History
	if limit <= 0 {
		limit = historyRecent
	}
	if limit > len(src) {
		limit = len(src)
	}
	if limit == 0 {
		return []HistoryEntry{}
	}
	return append([]HistoryEntry(nil), src[:limit]...)
}

func (s *Store) fillHistoryBook(e *HistoryEntry) {
	if e.Title != "" && e.Author != "" {
		return
	}
	for _, item := range s.ws().Library {
		if item.Key != e.BookKey {
			continue
		}
		if e.Title == "" {
			e.Title = item.Title
		}
		if e.Author == "" {
			e.Author = item.Author
		}
		return
	}
}

func (s *Store) AddHistory(e HistoryEntry) (HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.BookKey = strings.TrimSpace(e.BookKey)
	if e.BookKey == "" {
		return HistoryEntry{}, fmt.Errorf("книга нужна")
	}
	switch e.Kind {
	case HistorySession, HistoryNote:
	default:
		if strings.TrimSpace(e.Text) != "" {
			e.Kind = HistoryNote
		} else {
			e.Kind = HistorySession
		}
	}
	e.Text = clipRunes(e.Text, maxHistoryText)
	e.Title = clipRunes(e.Title, 200)
	e.Author = clipRunes(e.Author, 120)
	e.ChapterTitle = clipRunes(e.ChapterTitle, 200)
	if e.ScrollRatio < 0 {
		e.ScrollRatio = 0
	}
	if e.ScrollRatio > 1 {
		e.ScrollRatio = 1
	}
	if e.Kind == HistoryNote && e.Text == "" {
		return HistoryEntry{}, fmt.Errorf("нужен текст отметки")
	}
	s.fillHistoryBook(&e)
	now := time.Now()
	ws := s.ws()
	if e.Kind == HistorySession {
		for i, old := range ws.History {
			if old.Kind != HistorySession || old.BookKey != e.BookKey {
				continue
			}
			if now.Sub(old.CreatedAt) > sessionDedupWindow {
				break
			}
			old.Title = e.Title
			old.Author = e.Author
			old.ChapterIndex = e.ChapterIndex
			old.ChapterTitle = e.ChapterTitle
			old.ScrollRatio = e.ScrollRatio
			old.CreatedAt = now
			ws.History = append([]HistoryEntry{old}, append(ws.History[:i], ws.History[i+1:]...)...)
			ws.UpdatedAt = now
			return old, s.save()
		}
	}
	e.ID = historyID(e)
	e.CreatedAt = now
	ws.History = append([]HistoryEntry{e}, ws.History...)
	if len(ws.History) > maxHistoryKeep {
		ws.History = ws.History[:maxHistoryKeep]
	}
	ws.UpdatedAt = now
	return e, s.save()
}

func (s *Store) RemoveTodo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	var keep []Todo
	found := false
	for _, t := range ws.Todos {
		if t.ID == id {
			found = true
			continue
		}
		keep = append(keep, t)
	}
	if !found {
		return os.ErrNotExist
	}
	if keep == nil {
		keep = []Todo{}
	}
	ws.Todos = keep
	ws.UpdatedAt = time.Now()
	return s.save()
}

func todoID(t Todo) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("todo|%s|%s|%d", t.BookKey, t.Text, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) RemoveNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.ws()
	var keep []Note
	var removed Note
	found := false
	for _, n := range ws.Notes {
		if n.ID == id {
			found = true
			removed = n
			continue
		}
		keep = append(keep, n)
	}
	if !found {
		return os.ErrNotExist
	}
	ws.Notes = keep
	detail := strings.TrimSpace(removed.Body)
	if detail == "" {
		detail = strings.TrimSpace(removed.Text)
	}
	if detail == "" {
		detail = "Заметка"
	}
	s.pushUndoLocked(UndoAction{
		Kind:      UndoDeleteNote,
		Label:     "Удалена заметка",
		Detail:    clipRunes(detail, 80),
		BookKey:   removed.BookKey,
		BookTitle: s.bookTitleLocked(removed.BookKey),
		Note:      &removed,
	})
	return s.save()
}

func noteID(n Note) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("note|%s|%d|%d|%d|%d", n.BookKey, n.ChapterIndex, n.Start, n.End, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func highlightID(h Highlight) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d|%d", h.BookKey, h.ChapterIndex, h.Start, h.End, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func bookmarkID(b Bookmark) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%f|%d", b.BookKey, b.ChapterIndex, b.ScrollRatio, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:10])
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Store) SaveBookFile(key, name string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = ".epub"
	}
	dir := filepath.Join(s.dir, "library")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fileID(key)+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) SaveCover(key, mime string, data []byte) (string, error) {
	ext := ".img"
	switch {
	case strings.Contains(mime, "png"):
		ext = ".png"
	case strings.Contains(mime, "jpeg"), strings.Contains(mime, "jpg"):
		ext = ".jpg"
	case strings.Contains(mime, "webp"):
		ext = ".webp"
	case strings.Contains(mime, "gif"):
		ext = ".gif"
	}
	dir := filepath.Join(s.dir, "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fileID(key) + ext
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

func (s *Store) CoverFile(rel string) (string, error) {
	if rel == "" {
		return "", os.ErrNotExist
	}
	path := filepath.Join(s.dir, "covers", filepath.Base(rel))
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func fileID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:12])
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(s.path)
		if err := os.Rename(tmp, s.path); err != nil {
			_ = os.Remove(tmp)
			return os.WriteFile(s.path, raw, 0o644)
		}
	}
	return nil
}
