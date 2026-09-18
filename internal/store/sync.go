package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DriveDirName = "drive"
)

type SyncFile struct {
	Rel     string
	Abs     string
	Size    int64
	ModTime time.Time
}

var syncSubdirs = []string{"library", "covers", "dictionaries", "ui"}

func (s *Store) Persist() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = Data{UI: defaultUI()}
			s.ensureWorkspace()
			return nil
		}
		return err
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("не удалось прочитать настройки")
	}
	s.data = data
	s.data.UI = normalizeUI(s.data.UI)
	s.migrateWorkspaces()
	s.normalizeWorkspaces()
	s.ensureWorkspace()
	s.migrateLibrary()
	return nil
}

func (s *Store) SyncFiles() ([]SyncFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncFilesLocked()
}

func (s *Store) syncFilesLocked() ([]SyncFile, error) {
	var out []SyncFile
	if fi, err := os.Stat(s.path); err == nil && !fi.IsDir() {
		out = append(out, SyncFile{Rel: "state.json", Abs: s.path, Size: fi.Size(), ModTime: fi.ModTime()})
	}
	for _, sub := range syncSubdirs {
		dir := filepath.Join(s.dir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if skipSyncName(name) {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, SyncFile{
				Rel:     sub + "/" + name,
				Abs:     filepath.Join(dir, name),
				Size:    fi.Size(),
				ModTime: fi.ModTime(),
			})
		}
	}
	return out, nil
}

func skipSyncName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".drive-tmp")
}

func SafeSyncRel(rel string) (string, bool) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	if rel == "state.json" {
		return rel, true
	}
	slash := strings.IndexByte(rel, '/')
	if slash <= 0 {
		return "", false
	}
	sub := rel[:slash]
	base := rel[slash+1:]
	ok := false
	for _, name := range syncSubdirs {
		if sub == name {
			ok = true
			break
		}
	}
	if !ok || base == "" || strings.Contains(base, "/") || skipSyncName(base) {
		return "", false
	}
	return rel, true
}
