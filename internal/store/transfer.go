package store

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	TransferFormat     = "boo-library"
	TransferVersion    = 1
	MaxTransferBytes   = 4 << 30
	maxUncompressedZip = 8 << 30
)

type Manifest struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	Workspaces int       `json:"workspaces"`
	Books      int       `json:"books"`
	Files      int       `json:"files"`
}

func ExportFileName() string {
	return fmt.Sprintf("boo-library-%s.zip", time.Now().Format("2006-01-02"))
}

type transferFile struct {
	src  string
	dest string
}

func (s *Store) Export(w io.Writer) error {
	s.mu.Lock()
	snap, err := cloneData(s.data)
	files, books := s.collectTransferLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}

	prepareExportData(&snap, books)

	stateRaw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	man := Manifest{
		Format:     TransferFormat,
		Version:    TransferVersion,
		ExportedAt: time.Now(),
		Workspaces: len(snap.Workspaces),
		Books:      countBooks(snap),
		Files:      len(files),
	}
	manRaw, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	if err := addZipBytes(zw, "manifest.json", manRaw); err != nil {
		_ = zw.Close()
		return err
	}
	if err := addZipBytes(zw, "state.json", stateRaw); err != nil {
		_ = zw.Close()
		return err
	}
	for _, f := range files {
		if err := addZipFile(zw, f.dest, f.src); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func (s *Store) Import(r io.Reader) error {
	tmp, err := os.CreateTemp("", "boo-import-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, err := io.Copy(tmp, io.LimitReader(r, MaxTransferBytes+1))
	if err != nil {
		return err
	}
	if n > MaxTransferBytes {
		return fmt.Errorf("архив слишком большой")
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return err
	}
	zr, err := zip.NewReader(tmp, n)
	if err != nil {
		return fmt.Errorf("это не архив библиотеки boo")
	}
	return s.importZip(zr)
}

func (s *Store) importZip(zr *zip.Reader) error {
	var stateRaw, manRaw []byte
	var total uint64
	type pending struct {
		dir  string
		name string
		file *zip.File
	}
	var copies []pending
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := zipName(f.Name)
		if name == "" {
			return fmt.Errorf("архив повреждён")
		}
		if f.UncompressedSize64 > maxUncompressedZip {
			return fmt.Errorf("архив слишком большой")
		}
		total += f.UncompressedSize64
		if total > maxUncompressedZip {
			return fmt.Errorf("архив слишком большой")
		}
		switch {
		case name == "state.json":
			raw, err := readZipFile(f)
			if err != nil {
				return err
			}
			stateRaw = raw
		case name == "manifest.json":
			raw, err := readZipFile(f)
			if err != nil {
				return err
			}
			manRaw = raw
		case strings.HasPrefix(name, "library/"):
			base, ok := safeBase(strings.TrimPrefix(name, "library/"))
			if !ok {
				return fmt.Errorf("архив повреждён")
			}
			copies = append(copies, pending{dir: "library", name: base, file: f})
		case strings.HasPrefix(name, "covers/"):
			base, ok := safeBase(strings.TrimPrefix(name, "covers/"))
			if !ok {
				return fmt.Errorf("архив повреждён")
			}
			copies = append(copies, pending{dir: "covers", name: base, file: f})
		case strings.HasPrefix(name, "dictionaries/"):
			base, ok := safeBase(strings.TrimPrefix(name, "dictionaries/"))
			if !ok {
				return fmt.Errorf("архив повреждён")
			}
			copies = append(copies, pending{dir: "dictionaries", name: base, file: f})
		case strings.HasPrefix(name, "ui/"):
			base, ok := safeBase(strings.TrimPrefix(name, "ui/"))
			if !ok {
				return fmt.Errorf("архив повреждён")
			}
			copies = append(copies, pending{dir: "ui", name: base, file: f})
		}
	}
	if len(stateRaw) == 0 {
		return fmt.Errorf("в архиве нет данных библиотеки")
	}
	if len(manRaw) > 0 {
		var man Manifest
		if err := json.Unmarshal(manRaw, &man); err != nil {
			return fmt.Errorf("архив повреждён")
		}
		if man.Format != TransferFormat {
			return fmt.Errorf("это не архив библиотеки boo")
		}
		if man.Version < 1 || man.Version > TransferVersion {
			return fmt.Errorf("архив из более новой версии boo")
		}
	}
	var incoming Data
	if err := json.Unmarshal(stateRaw, &incoming); err != nil {
		return fmt.Errorf("архив повреждён")
	}

	keep := map[string]map[string]bool{
		"library":      {},
		"covers":       {},
		"dictionaries": {},
		"ui":           {},
	}
	for _, item := range copies {
		dir := filepath.Join(s.dir, item.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		dest := filepath.Join(dir, item.name)
		if err := extractZipFile(item.file, dest); err != nil {
			return err
		}
		keep[item.dir][item.name] = true
	}

	remapImportedPaths(&incoming, s.dir, keep["library"])
	incoming.UI = normalizeUI(incoming.UI)
	for i := range incoming.Workspaces {
		incoming.Workspaces[i].UndoLog = []UndoAction{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data
	s.data = incoming
	s.normalizeWorkspaces()
	s.ensureWorkspace()
	s.migrateLibrary()
	if err := s.save(); err != nil {
		s.data = old
		return err
	}
	for dir, names := range keep {
		s.removeUnlisted(dir, names)
	}
	_ = os.RemoveAll(filepath.Join(s.dir, "trash"))
	return nil
}

func (s *Store) collectTransferLocked() ([]transferFile, map[string]string) {
	var files []transferFile
	books := map[string]string{}
	seen := map[string]bool{}
	add := func(src, dest string) {
		if src == "" || dest == "" || seen[dest] {
			return
		}
		if _, err := os.Stat(src); err != nil {
			return
		}
		seen[dest] = true
		files = append(files, transferFile{src: src, dest: dest})
	}
	for _, ws := range s.data.Workspaces {
		for _, e := range ws.Library {
			if _, ok := books[e.Key]; !ok {
				if src, name, ok := s.findBookFileLocked(e); ok {
					books[e.Key] = name
					add(src, path.Join("library", name))
				}
			}
			if e.Cover != "" {
				src := filepath.Join(s.dir, "covers", filepath.Base(e.Cover))
				add(src, path.Join("covers", filepath.Base(e.Cover)))
			}
		}
	}
	for _, d := range s.data.Dictionaries {
		src := s.dictionaryPathLocked(d)
		add(src, path.Join("dictionaries", filepath.Base(src)))
	}
	if name := s.data.UI.WelcomeBackground; name != "" {
		src := filepath.Join(s.dir, "ui", filepath.Base(name))
		add(src, path.Join("ui", filepath.Base(name)))
	}
	return files, books
}

func (s *Store) findBookFileLocked(e Entry) (string, string, bool) {
	if BundledFormat(e.Format) {
		return "", "", false
	}
	if e.Path != "" {
		if fi, err := os.Stat(e.Path); err == nil && !fi.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Path))
			if ext == "" {
				ext = ".epub"
			}
			return e.Path, fileID(e.Key) + ext, true
		}
	}
	matches, _ := filepath.Glob(filepath.Join(s.dir, "library", fileID(e.Key)+".*"))
	for _, p := range matches {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, filepath.Base(p), true
		}
	}
	return "", "", false
}

func prepareExportData(data *Data, books map[string]string) {
	data.Books = nil
	data.Library = nil
	data.Bookmarks = nil
	data.Highlights = nil
	data.Notes = nil
	data.TOCFold = nil
	for i := range data.Workspaces {
		ws := &data.Workspaces[i]
		ws.UndoLog = []UndoAction{}
		for j := range ws.Library {
			e := &ws.Library[j]
			if name, ok := books[e.Key]; ok {
				e.Path = name
				continue
			}
			if !BundledFormat(e.Format) {
				e.Path = ""
			}
		}
	}
}

func remapImportedPaths(data *Data, dir string, libraryFiles map[string]bool) {
	libDir := filepath.Join(dir, "library")
	for i := range data.Workspaces {
		ws := &data.Workspaces[i]
		for j := range ws.Library {
			e := &ws.Library[j]
			if BundledFormat(e.Format) {
				e.Path = ""
				continue
			}
			name := filepath.Base(strings.ReplaceAll(e.Path, "\\", "/"))
			if name == "" || name == "." || name == ".." {
				if files := libraryFiles; files != nil {
					want := fileID(e.Key)
					for existing := range files {
						if strings.HasPrefix(existing, want) {
							name = existing
							break
						}
					}
				}
			}
			if name == "" || name == "." || name == ".." || (libraryFiles != nil && !libraryFiles[name]) {
				e.Path = ""
				continue
			}
			e.Path = filepath.Join(libDir, name)
		}
	}
}

func (s *Store) removeUnlisted(subdir string, keep map[string]bool) {
	dir := filepath.Join(s.dir, subdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if keep[e.Name()] {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

func countBooks(data Data) int {
	seen := map[string]bool{}
	n := 0
	for _, ws := range data.Workspaces {
		for _, e := range ws.Library {
			if seen[e.Key] {
				continue
			}
			seen[e.Key] = true
			n++
		}
	}
	return n
}

func cloneData(d Data) (Data, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return Data{}, err
	}
	var out Data
	if err := json.Unmarshal(raw, &out); err != nil {
		return Data{}, err
	}
	return out, nil
}

func addZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func addZipFile(zw *zip.Writer, name, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	limit := int64(f.UncompressedSize64) + 1
	if limit < 1 {
		limit = 1
	}
	data, err := io.ReadAll(io.LimitReader(rc, limit))
	if err != nil {
		return nil, err
	}
	if uint64(len(data)) > f.UncompressedSize64 && f.UncompressedSize64 > 0 {
		return nil, fmt.Errorf("архив повреждён")
	}
	return data, nil
}

func extractZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	limit := int64(f.UncompressedSize64) + 1
	if limit < 1 {
		limit = 1
	}
	_, copyErr := io.Copy(out, io.LimitReader(rc, limit))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func zipName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimPrefix(name, "./")
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return ""
	}
	return name
}

func safeBase(name string) (string, bool) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", false
	}
	base := path.Base(name)
	if base == "" || base == "." || base == ".." {
		return "", false
	}
	for _, r := range base {
		if r < 32 {
			return "", false
		}
	}
	return base, true
}
