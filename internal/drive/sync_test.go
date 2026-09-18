package drive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"boo/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSyncUploadDownloadAndNewerWins(t *testing.T) {
	st := testStore(t)
	if err := st.Persist(); err != nil {
		t.Fatal(err)
	}
	bookPath, err := st.SaveBookFile("id:1", "a.fb2", []byte("local-book"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(store.Entry{Key: "id:1", Title: "Книга", Path: bookPath, Format: "fb2"}); err != nil {
		t.Fatal(err)
	}
	svc := NewMemory(st.Dir())
	ctx := context.Background()
	res, err := svc.Sync(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if res.Uploaded < 2 {
		t.Fatalf("uploaded %#v", res)
	}
	mem := svc.backend.(*memBackend)
	if n := countRemoteFiles(mem); n < 2 {
		t.Fatalf("remote files %d", n)
	}

	old := time.Now().Add(-time.Hour)
	newer := time.Now().Add(time.Hour)
	libID, err := mem.EnsureDir(ctx, mustRoot(t, mem), "library")
	if err != nil {
		t.Fatal(err)
	}
	mem.seed(libID, "from-cloud.fb2", []byte("cloud-book"), newer)
	rootID := mustRoot(t, mem)
	stateID := remoteID(mem, rootID, "state.json")
	if stateID == "" {
		t.Fatal("no remote state")
	}
	raw, err := os.ReadFile(filepath.Join(st.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data store.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	data.Workspaces[0].Name = "С Диска"
	out, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mem.Update(ctx, stateID, newer, strings.NewReader(string(out)), int64(len(out))); err != nil {
		t.Fatal(err)
	}
	_ = old

	res, err = svc.Sync(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if res.Downloaded < 1 {
		t.Fatalf("expected download %#v", res)
	}
	got, err := os.ReadFile(filepath.Join(st.Dir(), "library", "from-cloud.fb2"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "cloud-book" {
		t.Fatalf("book %q", got)
	}
	if st.CurrentWorkspace().Name != "С Диска" {
		t.Fatalf("state reload %#v", st.CurrentWorkspace())
	}
}

func TestSyncDoesNotDeleteRemote(t *testing.T) {
	st := testStore(t)
	if err := st.Persist(); err != nil {
		t.Fatal(err)
	}
	svc := NewMemory(st.Dir())
	ctx := context.Background()
	if _, err := svc.Sync(ctx, st); err != nil {
		t.Fatal(err)
	}
	mem := svc.backend.(*memBackend)
	libID, err := mem.EnsureDir(ctx, mustRoot(t, mem), "library")
	if err != nil {
		t.Fatal(err)
	}
	mem.seed(libID, "kept.fb2", []byte("keep-me"), time.Now())
	bookPath, err := st.SaveBookFile("id:1", "a.fb2", []byte("local-only"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(store.Entry{Key: "id:1", Title: "Книга", Path: bookPath, Format: "fb2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sync(ctx, st); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(st.Dir(), "library", "kept.fb2")); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	before := countRemoteFiles(mem)
	if _, err := svc.Sync(ctx, st); err != nil {
		t.Fatal(err)
	}
	after := countRemoteFiles(mem)
	if after < before {
		t.Fatalf("remote deleted %d -> %d", before, after)
	}
}

func TestSyncSkipsTokenAndDriveDir(t *testing.T) {
	st := testStore(t)
	if err := st.Persist(); err != nil {
		t.Fatal(err)
	}
	tokenDir := filepath.Join(st.Dir(), store.DriveDirName)
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "token.json"), []byte(`{"access_token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewMemory(st.Dir())
	if _, err := svc.Sync(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	mem := svc.backend.(*memBackend)
	rootID := mustRoot(t, mem)
	files, err := mem.List(context.Background(), rootID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Name), "token") {
			t.Fatalf("token uploaded %#v", f)
		}
	}
}

func TestSaveCredentials(t *testing.T) {
	dir := t.TempDir()
	svc := New(dir)
	if svc.HasCredentials() || svc.Connected() {
		t.Fatal("empty")
	}
	if err := svc.SaveCredentials("", "x"); err == nil {
		t.Fatal("empty id")
	}
	if err := svc.SaveCredentials("id", "secret"); err != nil {
		t.Fatal(err)
	}
	if !svc.HasCredentials() {
		t.Fatal("expected creds")
	}
	if svc.Connected() {
		t.Fatal("not connected")
	}
}

func TestSyncRequiresConnection(t *testing.T) {
	st := testStore(t)
	svc := New(st.Dir())
	_, err := svc.Sync(context.Background(), st)
	if err == nil || !strings.Contains(err.Error(), "ключ") {
		t.Fatalf("err %v", err)
	}
	if err := svc.SaveCredentials("id", "secret"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Sync(context.Background(), st)
	if err == nil || !strings.Contains(err.Error(), "не подключён") {
		t.Fatalf("err %v", err)
	}
}

func TestStartConnectURL(t *testing.T) {
	dir := t.TempDir()
	svc := New(dir)
	if err := svc.SaveCredentials("client-id", "client-secret"); err != nil {
		t.Fatal(err)
	}
	svc.openURL = func(string) error { return nil }
	authURL, err := svc.StartConnect("http://127.0.0.1:7474/api/drive/callback")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authURL, "accounts.google.com") || !strings.Contains(authURL, "client-id") {
		t.Fatalf("url %s", authURL)
	}
	if err := svc.FinishConnect(context.Background(), "wrong", "code"); err == nil {
		t.Fatal("expected stale login")
	}
}

func mustRoot(t *testing.T, mem *memBackend) string {
	t.Helper()
	id, err := mem.EnsureDir(context.Background(), "root", rootName)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func remoteID(mem *memBackend, parent, name string) string {
	files, _ := mem.List(context.Background(), parent)
	for _, f := range files {
		if f.Name == name {
			return f.ID
		}
	}
	return ""
}

func countRemoteFiles(mem *memBackend) int {
	n := 0
	var walk func(string)
	walk = func(parent string) {
		files, _ := mem.List(context.Background(), parent)
		for _, f := range files {
			if f.Folder {
				walk(f.ID)
				continue
			}
			n++
		}
	}
	walk("root")
	return n
}
