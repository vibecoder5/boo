package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"boo/internal/epub"
)

func TestScanFolderAddsToBacklog(t *testing.T) {
	root := t.TempDir()
	epubData, err := epub.Sample()
	if err != nil {
		t.Fatal(err)
	}
	epubPath := filepath.Join(root, "story.epub")
	if err := os.WriteFile(epubPath, epubData, 0o644); err != nil {
		t.Fatal(err)
	}
	fb2Path := filepath.Join(root, "summer.fb2")
	if err := os.WriteFile(fb2Path, []byte(scanFB2), 0o644); err != nil {
		t.Fatal(err)
	}
	txtPath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("Глава\n\nТекст.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Заметка\n\nТекст.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	srv := New(st, fs.FS(ui), nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	var scanned struct {
		Workspace string `json:"workspace"`
		Books     []struct {
			Path   string `json:"path"`
			Name   string `json:"name"`
			Title  string `json:"title"`
			Format string `json:"format"`
		} `json:"books"`
	}
	postJSON(t, ts.URL+"/api/library/scan", map[string]string{"path": root}, &scanned)
	if scanned.Workspace != "Backlog" {
		t.Fatalf("workspace %q", scanned.Workspace)
	}
	if len(scanned.Books) != 2 {
		t.Fatalf("books %#v", scanned.Books)
	}
	for _, b := range scanned.Books {
		if b.Name == "notes.txt" || b.Name == "notes.md" {
			t.Fatalf("text scanned %#v", scanned.Books)
		}
		if b.Format != "epub" && b.Format != "fb2" {
			t.Fatalf("format %s", b.Format)
		}
	}

	var added struct {
		Added     int    `json:"added"`
		Skipped   int    `json:"skipped"`
		Failed    int    `json:"failed"`
		Workspace string `json:"workspace"`
	}
	postJSON(t, ts.URL+"/api/library/scan/add", map[string]any{
		"paths": []string{epubPath, fb2Path, txtPath},
	}, &added)
	if added.Added != 2 || added.Skipped != 0 || added.Failed != 1 || added.Workspace != "Backlog" {
		t.Fatalf("add %#v", added)
	}

	state := getState(t, ts.URL)
	if state.Workspace.Name != "Библиотека" || state.Workspace.Current != true {
		t.Fatalf("current %#v", state.Workspace)
	}
	if len(state.Library) != 0 {
		t.Fatalf("current shelf %#v", state.Library)
	}
	backlog := findWorkspace(t, state, "Backlog")
	if backlog.BookCount != 2 || backlog.Current {
		t.Fatalf("backlog %#v", backlog)
	}

	postJSON(t, ts.URL+"/api/library/scan/add", map[string]any{
		"paths": []string{epubPath},
	}, &added)
	if added.Added != 0 || added.Skipped != 1 {
		t.Fatalf("second %#v", added)
	}

	uiSettings := st.UI()
	uiSettings.ImportWorkspace = "Очередь"
	if err := st.SetUI(uiSettings); err != nil {
		t.Fatal(err)
	}
	postJSON(t, ts.URL+"/api/library/scan/add", map[string]any{
		"paths": []string{epubPath},
	}, &added)
	if added.Added != 1 || added.Workspace != "Очередь" {
		t.Fatalf("custom %#v", added)
	}
	state = getState(t, ts.URL)
	if state.Workspace.Name != "Библиотека" {
		t.Fatalf("switched to %s", state.Workspace.Name)
	}
	queue := findWorkspace(t, state, "Очередь")
	if queue.BookCount != 1 {
		t.Fatalf("queue %#v", queue)
	}
	if findWorkspace(t, state, "Backlog").BookCount != 2 {
		t.Fatal("backlog changed")
	}

	missing, err := json.Marshal(map[string]string{"path": filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/library/scan", "application/json", bytes.NewReader(missing))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing folder %d", res.StatusCode)
	}
}

type scanState struct {
	Workspace struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
	} `json:"workspace"`
	Workspaces []struct {
		Name      string `json:"name"`
		BookCount int    `json:"bookCount"`
		Current   bool   `json:"current"`
	} `json:"workspaces"`
	Library []map[string]any `json:"library"`
}

func postJSON(t *testing.T, url string, body any, dest any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		t.Fatalf("%s: %d %s", url, res.StatusCode, buf.String())
	}
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		t.Fatal(err)
	}
}

func getState(t *testing.T, base string) scanState {
	t.Helper()
	res, err := http.Get(base + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("state %d", res.StatusCode)
	}
	var state scanState
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func findWorkspace(t *testing.T, state scanState, name string) struct {
	Name      string `json:"name"`
	BookCount int    `json:"bookCount"`
	Current   bool   `json:"current"`
} {
	t.Helper()
	for _, ws := range state.Workspaces {
		if ws.Name == name {
			return ws
		}
	}
	t.Fatalf("no workspace %s in %#v", name, state.Workspaces)
	return state.Workspaces[0]
}

const scanFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
  <description>
    <title-info>
      <book-title>Летний сад</book-title>
      <author><first-name>Анна</first-name><last-name>Речная</last-name></author>
      <lang>ru</lang>
    </title-info>
  </description>
  <body>
    <section>
      <title><p>Глава</p></title>
      <p>Текст.</p>
    </section>
  </body>
</FictionBook>
`
