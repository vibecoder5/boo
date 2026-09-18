package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"boo/internal/store"

	"golang.org/x/oauth2"
)

const (
	folderMIME = "application/vnd.google-apps.folder"
	rootName   = "boo"
	timeSkew   = 2 * time.Second
)

type Service struct {
	dir        string
	mu         sync.Mutex
	pending    *pendingAuth
	backend    Backend
	openURL    func(string) error
	apiBase    string
	uploadBase string
}

type Status struct {
	Connected      bool       `json:"connected"`
	HasCredentials bool       `json:"hasCredentials"`
	Email          string     `json:"email,omitempty"`
	LastSync       *time.Time `json:"lastSync,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	Uploaded       int        `json:"uploaded,omitempty"`
	Downloaded     int        `json:"downloaded,omitempty"`
	Skipped        int        `json:"skipped,omitempty"`
}

type Result struct {
	Uploaded   int
	Downloaded int
	Skipped    int
	Email      string
}

type persistedStatus struct {
	Email      string    `json:"email,omitempty"`
	LastSync   time.Time `json:"lastSync,omitempty"`
	LastError  string    `json:"lastError,omitempty"`
	Uploaded   int       `json:"uploaded,omitempty"`
	Downloaded int       `json:"downloaded,omitempty"`
	Skipped    int       `json:"skipped,omitempty"`
}

func New(dir string) *Service {
	return &Service{
		dir:        dir,
		openURL:    openBrowser,
		apiBase:    "https://www.googleapis.com",
		uploadBase: "https://www.googleapis.com/upload",
	}
}

func NewMemory(dir string) *Service {
	s := New(dir)
	s.backend = newMemBackend()
	s.openURL = func(string) error { return nil }
	_ = s.saveToken(&oauth2.Token{AccessToken: "test", TokenType: "Bearer"})
	return s
}

func (s *Service) Status() Status {
	saved := s.loadPersisted()
	st := Status{
		Connected:      s.Connected(),
		HasCredentials: s.HasCredentials(),
		Email:          saved.Email,
		LastError:      saved.LastError,
		Uploaded:       saved.Uploaded,
		Downloaded:     saved.Downloaded,
		Skipped:        saved.Skipped,
	}
	if !saved.LastSync.IsZero() {
		t := saved.LastSync
		st.LastSync = &t
	}
	return st
}

func (s *Service) Connected() bool {
	tok, err := s.loadToken()
	return err == nil && tok != nil && (tok.Valid() || tok.RefreshToken != "")
}

func (s *Service) driveDir() string {
	return filepath.Join(s.dir, store.DriveDirName)
}

func (s *Service) statusPath() string {
	return filepath.Join(s.driveDir(), "status.json")
}

func (s *Service) loadPersisted() persistedStatus {
	raw, err := os.ReadFile(s.statusPath())
	if err != nil {
		return persistedStatus{}
	}
	var st persistedStatus
	if json.Unmarshal(raw, &st) != nil {
		return persistedStatus{}
	}
	return st
}

func (s *Service) savePersisted(st persistedStatus) {
	if err := os.MkdirAll(s.driveDir(), 0o700); err != nil {
		return
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.statusPath(), raw, 0o600)
}

func (s *Service) rememberError(msg string) {
	st := s.loadPersisted()
	st.LastError = msg
	s.savePersisted(st)
}

func (s *Service) getBackend(ctx context.Context) (Backend, error) {
	if !s.Connected() {
		if !s.HasCredentials() && s.backend == nil {
			return nil, fmt.Errorf("сначала сохраните ключ приложения Google")
		}
		return nil, fmt.Errorf("Google Диск не подключён")
	}
	if s.backend != nil {
		return s.backend, nil
	}
	if !s.HasCredentials() {
		return nil, fmt.Errorf("сначала сохраните ключ приложения Google")
	}
	client, err := s.httpClient(ctx)
	if err != nil {
		return nil, err
	}
	return &restBackend{http: client, api: s.apiBase, upload: s.uploadBase}, nil
}
