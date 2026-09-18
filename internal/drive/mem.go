package drive

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"
)

type memBackend struct {
	mu      sync.Mutex
	seq     int
	email   string
	files   map[string]*memFile
	byChild map[string][]string
}

type memFile struct {
	remoteFile
	parent string
	data   []byte
}

func newMemBackend() *memBackend {
	root := &memFile{remoteFile: remoteFile{ID: "root", Name: "root", Folder: true}}
	return &memBackend{
		email:   "user@example.com",
		files:   map[string]*memFile{"root": root},
		byChild: map[string][]string{},
	}
}

func (m *memBackend) nextID() string {
	m.seq++
	return "id-" + strconv.Itoa(m.seq)
}

func (m *memBackend) About(ctx context.Context) (string, error) {
	return m.email, nil
}

func (m *memBackend) EnsureDir(ctx context.Context, parentID, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if parentID == "" {
		parentID = "root"
	}
	if m.files[parentID] == nil {
		return "", fmt.Errorf("Google Диск: нет папки")
	}
	for _, id := range m.byChild[parentID] {
		f := m.files[id]
		if f != nil && f.Folder && f.Name == name {
			return f.ID, nil
		}
	}
	id := m.nextID()
	f := &memFile{
		remoteFile: remoteFile{ID: id, Name: name, Folder: true, ModTime: time.Now().UTC()},
		parent:     parentID,
	}
	m.files[id] = f
	m.byChild[parentID] = append(m.byChild[parentID], id)
	return id, nil
}

func (m *memBackend) List(ctx context.Context, parentID string) ([]remoteFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if parentID == "" {
		parentID = "root"
	}
	var out []remoteFile
	for _, id := range m.byChild[parentID] {
		if f := m.files[id]; f != nil {
			cp := f.remoteFile
			out = append(out, cp)
		}
	}
	return out, nil
}

func (m *memBackend) Upload(ctx context.Context, parentID, name string, mod time.Time, r io.Reader, size int64) (remoteFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return remoteFile{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if parentID == "" {
		parentID = "root"
	}
	id := m.nextID()
	sum := md5.Sum(data)
	if mod.IsZero() {
		mod = time.Now()
	}
	f := &memFile{
		remoteFile: remoteFile{
			ID:      id,
			Name:    name,
			MD5:     hex.EncodeToString(sum[:]),
			Size:    int64(len(data)),
			ModTime: mod.UTC(),
		},
		parent: parentID,
		data:   data,
	}
	m.files[id] = f
	m.byChild[parentID] = append(m.byChild[parentID], id)
	return f.remoteFile, nil
}

func (m *memBackend) Update(ctx context.Context, id string, mod time.Time, r io.Reader, size int64) (remoteFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return remoteFile{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.files[id]
	if f == nil || f.Folder {
		return remoteFile{}, fmt.Errorf("Google Диск: файл не найден")
	}
	sum := md5.Sum(data)
	if mod.IsZero() {
		mod = time.Now()
	}
	f.data = data
	f.MD5 = hex.EncodeToString(sum[:])
	f.Size = int64(len(data))
	f.ModTime = mod.UTC()
	return f.remoteFile, nil
}

func (m *memBackend) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.files[id]
	if f == nil || f.Folder {
		return nil, fmt.Errorf("Google Диск: файл не найден")
	}
	cp := append([]byte(nil), f.data...)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (m *memBackend) seed(parentID, name string, data []byte, mod time.Time) remoteFile {
	m.mu.Lock()
	defer m.mu.Unlock()
	if parentID == "" {
		parentID = "root"
	}
	id := m.nextID()
	sum := md5.Sum(data)
	if mod.IsZero() {
		mod = time.Now()
	}
	f := &memFile{
		remoteFile: remoteFile{
			ID:      id,
			Name:    name,
			MD5:     hex.EncodeToString(sum[:]),
			Size:    int64(len(data)),
			ModTime: mod.UTC(),
		},
		parent: parentID,
		data:   append([]byte(nil), data...),
	}
	m.files[id] = f
	m.byChild[parentID] = append(m.byChild[parentID], id)
	return f.remoteFile
}
