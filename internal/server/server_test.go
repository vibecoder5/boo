package server

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"boo/internal/epub"
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

func TestDemoChapterAPI(t *testing.T) {
	data, err := epub.Sample()
	if err != nil {
		t.Fatal(err)
	}
	book, err := epub.OpenBytes("demo.epub", data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })

	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	srv := New(testStore(t), fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("state: %d", res.StatusCode)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] == nil {
		t.Fatal("expected book in state")
	}

	res, err = http.Get(ts.URL + "/api/chapter?i=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var ch map[string]any
	if err := json.NewDecoder(res.Body).Decode(&ch); err != nil {
		t.Fatal(err)
	}
	html, _ := ch["html"].(string)
	if strings.Contains(html, "alert") {
		t.Fatal("script leaked through API")
	}
	if !strings.Contains(html, "клавиатуре") {
		t.Fatalf("unexpected chapter: %s", html)
	}

	res, err = http.Get(ts.URL + "/api/search?q=" + url.QueryEscape("оглавление"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var found map[string]any
	if err := json.NewDecoder(res.Body).Decode(&found); err != nil {
		t.Fatal(err)
	}
	hits, _ := found["hits"].([]any)
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}

	res, err = http.Post(ts.URL+"/api/highlights", "application/json", strings.NewReader(
		`{"chapterIndex":1,"start":4,"end":18,"color":"green","text":"фрагмент"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("highlight: %d", res.StatusCode)
	}
	var hl map[string]any
	if err := json.NewDecoder(res.Body).Decode(&hl); err != nil {
		t.Fatal(err)
	}
	list, _ := hl["highlights"].([]any)
	if len(list) != 1 {
		t.Fatalf("highlights %#v", hl)
	}

	res, err = http.Post(ts.URL+"/api/notes", "application/json", strings.NewReader(
		`{"chapterIndex":1,"start":4,"end":18,"color":"blue","text":"фрагмент","body":"заметка"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("note: %d", res.StatusCode)
	}
	var notes map[string]any
	if err := json.NewDecoder(res.Body).Decode(&notes); err != nil {
		t.Fatal(err)
	}
	nlist, _ := notes["notes"].([]any)
	if len(nlist) != 1 {
		t.Fatalf("notes %#v", notes)
	}
}

func TestGuideAPI(t *testing.T) {
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	srv := New(testStore(t), fs.FS(ui), nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/guide", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("guide: %d %s", res.StatusCode, body)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	book, _ := state["book"].(map[string]any)
	if book == nil {
		t.Fatal("expected book")
	}
	if book["format"] != "guide" {
		t.Fatalf("format %#v", book["format"])
	}
	if book["key"] != "id:"+epub.GuideIdentifier {
		t.Fatalf("key %#v", book["key"])
	}
	lib, _ := state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("library %#v", lib)
	}
	item, _ := lib[0].(map[string]any)
	if item["canOpen"] != true {
		t.Fatalf("canOpen %#v", item)
	}

	res, err = http.Get(ts.URL + "/api/chapter?i=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var ch map[string]any
	if err := json.NewDecoder(res.Body).Decode(&ch); err != nil {
		t.Fatal(err)
	}
	html, _ := ch["html"].(string)
	if !strings.Contains(html, "boo") {
		t.Fatalf("chapter: %s", html)
	}

	res, err = http.Post(ts.URL+"/api/close", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	res, err = http.Post(ts.URL+"/api/library/open", "application/json", strings.NewReader(
		`{"key":"id:`+epub.GuideIdentifier+`"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("reopen: %d %s", res.StatusCode, body)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	book, _ = state["book"].(map[string]any)
	if book == nil || book["format"] != "guide" {
		t.Fatalf("reopen %#v", state["book"])
	}
}

func TestWorkspaceAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] == nil {
		t.Fatal("book")
	}
	ws, _ := state["workspace"].(map[string]any)
	if ws == nil || ws["id"] == "" {
		t.Fatalf("workspace %#v", state["workspace"])
	}
	firstID, _ := ws["id"].(string)

	res, err = http.Post(ts.URL+"/api/workspaces", "application/json", strings.NewReader(`{"name":"Учёба"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("create: %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("create should close book")
	}
	lib, _ := state["library"].([]any)
	if len(lib) != 0 {
		t.Fatalf("new workspace library %#v", lib)
	}
	next, _ := state["workspace"].(map[string]any)
	if next["name"] != "Учёба" {
		t.Fatalf("name %#v", next)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/workspaces/current", strings.NewReader(`{"id":"`+firstID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("switch: %d", out.StatusCode)
	}
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ := state["workspace"].(map[string]any)
	if cur["id"] != firstID {
		t.Fatalf("switched %#v", cur)
	}
	lib, _ = state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("first library %#v", lib)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/workspaces", strings.NewReader(`{"id":"`+firstID+`","name":"Дом"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("rename: %d", out.StatusCode)
	}
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ = state["workspace"].(map[string]any)
	if cur["id"] != firstID || cur["name"] != "Дом" {
		t.Fatalf("renamed current %#v", cur)
	}
}

func TestMoveBookAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	ws, _ := state["workspace"].(map[string]any)
	firstID, _ := ws["id"].(string)
	lib, _ := state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("library %#v", lib)
	}
	item, _ := lib[0].(map[string]any)
	key, _ := item["key"].(string)
	if firstID == "" || key == "" {
		t.Fatalf("ids %#v key %q", ws, key)
	}

	res, err = http.Post(ts.URL+"/api/workspaces", "application/json", strings.NewReader(`{"name":"Учёба"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	next, _ := state["workspace"].(map[string]any)
	secondID, _ := next["id"].(string)
	if secondID == "" || secondID == firstID {
		t.Fatalf("second %#v", next)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/workspaces/current", strings.NewReader(`{"id":"`+firstID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}

	res, err = http.Post(ts.URL+"/api/library/open", "application/json", strings.NewReader(`{"key":"`+key+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("open: %d", res.StatusCode)
	}

	res, err = http.Post(ts.URL+"/api/library/move", "application/json", strings.NewReader(`{"key":"`+key+`","id":"`+secondID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("move: %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("moved open book should close")
	}
	lib, _ = state["library"].([]any)
	if len(lib) != 0 {
		t.Fatalf("source library %#v", lib)
	}
	cur, _ := state["workspace"].(map[string]any)
	if cur["id"] != firstID {
		t.Fatalf("should stay %#v", cur)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/workspaces/current", strings.NewReader(`{"id":"`+secondID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lib, _ = state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("dest library %#v", lib)
	}
	moved, _ := lib[0].(map[string]any)
	if moved["key"] != key {
		t.Fatalf("moved %#v", moved)
	}

	res, err = http.Post(ts.URL+"/api/library/move", "application/json", strings.NewReader(`{"key":"`+key+`","name":"Дача"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("move create: %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lib, _ = state["library"].([]any)
	if len(lib) != 0 {
		t.Fatalf("second source %#v", lib)
	}
	cur, _ = state["workspace"].(map[string]any)
	if cur["id"] != secondID {
		t.Fatalf("stay after create %#v", cur)
	}
	names := map[string]bool{}
	for _, raw := range state["workspaces"].([]any) {
		item, _ := raw.(map[string]any)
		names[item["name"].(string)] = true
	}
	if !names["Дача"] {
		t.Fatalf("created workspace %#v", state["workspaces"])
	}
}

func TestReadingListAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lib, _ := state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("library %#v", lib)
	}
	item, _ := lib[0].(map[string]any)
	key, _ := item["key"].(string)
	if key == "" {
		t.Fatal("book key")
	}

	res, err = http.Post(ts.URL+"/api/lists", "application/json", strings.NewReader(`{"name":"На отпуск"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("create: %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lists, _ := state["lists"].([]any)
	if len(lists) != 1 {
		t.Fatalf("lists %#v", state["lists"])
	}
	created, _ := lists[0].(map[string]any)
	if created["name"] != "На отпуск" {
		t.Fatalf("name %#v", created)
	}
	listID, _ := created["id"].(string)

	res, err = http.Post(ts.URL+"/api/lists/books", "application/json", strings.NewReader(`{"id":"`+listID+`","key":"`+key+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("add: %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lists, _ = state["lists"].([]any)
	added, _ := lists[0].(map[string]any)
	if int(added["bookCount"].(float64)) != 1 {
		t.Fatalf("count %#v", added)
	}

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/lists/books?id="+url.QueryEscape(listID)+"&key="+url.QueryEscape(key), nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("remove book: %d", out.StatusCode)
	}
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lists, _ = state["lists"].([]any)
	cleared, _ := lists[0].(map[string]any)
	if int(cleared["bookCount"].(float64)) != 0 {
		t.Fatalf("cleared %#v", cleared)
	}

	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/api/lists?id="+url.QueryEscape(listID), nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("delete list: %d", out.StatusCode)
	}
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	lists, _ = state["lists"].([]any)
	if len(lists) != 0 {
		t.Fatalf("left %#v", lists)
	}
}

func TestFinishedAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/library/finished", strings.NewReader(`{"finished":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("finished: %d", res.StatusCode)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ := state["book"].(map[string]any)
	if cur["finished"] != true {
		t.Fatalf("book %#v", cur)
	}
	lib, _ := state["library"].([]any)
	if len(lib) == 0 {
		t.Fatal("library")
	}
	item, _ := lib[0].(map[string]any)
	if item["finished"] != true {
		t.Fatalf("library item %#v", item)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/library/finished", strings.NewReader(`{"key":"`+item["key"].(string)+`","finished":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ = state["book"].(map[string]any)
	item, _ = state["library"].([]any)[0].(map[string]any)
	if cur["finished"] != false || item["finished"] != false {
		t.Fatalf("unset book %#v item %#v", cur, item)
	}
}

func TestBookMetaAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/library/meta", strings.NewReader(`{"description":"о книге","journal":"длинная мысль"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("meta: %d", res.StatusCode)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ := state["book"].(map[string]any)
	if cur["description"] != "о книге" || cur["journal"] != "длинная мысль" {
		t.Fatalf("book %#v", cur)
	}
	lib, _ := state["library"].([]any)
	if len(lib) == 0 {
		t.Fatal("library")
	}
	item, _ := lib[0].(map[string]any)
	if item["description"] != "о книге" || item["journal"] != "длинная мысль" {
		t.Fatalf("library item %#v", item)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/library/meta", strings.NewReader(`{"key":"`+item["key"].(string)+`","description":"","journal":""}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if err := json.NewDecoder(out.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ = state["book"].(map[string]any)
	item, _ = state["library"].([]any)[0].(map[string]any)
	if cur["description"] != "" || item["journal"] != "" {
		t.Fatalf("unset book %#v item %#v", cur, item)
	}
}

func TestChapterReadAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/chapters/read", strings.NewReader(`{"chapterIndex":1,"read":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("read: %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	list, _ := body["readChapters"].([]any)
	if len(list) != 1 || list[0] != float64(1) {
		t.Fatalf("read chapters %#v", body)
	}

	stateRes, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer stateRes.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(stateRes.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	list, _ = state["readChapters"].([]any)
	if len(list) != 1 || list[0] != float64(1) {
		t.Fatalf("state %#v", state["readChapters"])
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/chapters/read", strings.NewReader(`{"chapterIndex":99,"read":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	bad, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad chapter: %d", bad.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/chapters/read", strings.NewReader(`{"chapterIndex":1,"read":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if err := json.NewDecoder(out.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	list, _ = body["readChapters"].([]any)
	if len(list) != 0 {
		t.Fatalf("unset %#v", body)
	}

	res, err = http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	list, _ = state["readChapters"].([]any)
	if len(list) != 0 {
		t.Fatalf("state %#v", state["readChapters"])
	}
}

func TestOpenLastBook(t *testing.T) {
	st := testStore(t)
	if OpenLast(st) != nil {
		t.Fatal("empty library")
	}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	_ = New(st, fs.FS(ui), book)
	last := OpenLast(st)
	if last == nil {
		t.Fatal("expected last book")
	}
	t.Cleanup(func() { _ = last.Close() })
	if last.Title != book.Title {
		t.Fatalf("title %q", last.Title)
	}

	srv := New(st, fs.FS(ui), nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("New(nil) should not auto-open; main restores last book")
	}
}

func TestTodosAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	cur, _ := state["book"].(map[string]any)
	key, _ := cur["key"].(string)
	if key == "" {
		t.Fatalf("book %#v", cur)
	}
	todoBook, _ := state["todoBook"].(map[string]any)
	if todoBook["key"] != key {
		t.Fatalf("todoBook %#v", todoBook)
	}
	if list, _ := state["todos"].([]any); len(list) != 0 {
		t.Fatalf("todos %#v", list)
	}

	res, err = http.Post(ts.URL+"/api/todos", "application/json", strings.NewReader(
		`{"text":"дочитать главу","dueAt":"2026-09-13T18:00"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("add: %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	list, _ := payload["todos"].([]any)
	if len(list) != 1 {
		t.Fatalf("todos %#v", payload)
	}
	item, _ := list[0].(map[string]any)
	id, _ := item["id"].(string)
	if id == "" || item["text"] != "дочитать главу" || item["done"] == true {
		t.Fatalf("item %#v", item)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/todos", strings.NewReader(
		`{"id":"`+id+`","text":"дочитать главу","dueAt":"2026-09-13T18:00","done":true}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("update: %d", out.StatusCode)
	}
	if err := json.NewDecoder(out.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	item, _ = payload["todos"].([]any)[0].(map[string]any)
	if item["done"] != true {
		t.Fatalf("done %#v", item)
	}

	del, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/todos?id="+url.QueryEscape(id)+"&key="+url.QueryEscape(key), nil)
	if err != nil {
		t.Fatal(err)
	}
	gone, err := http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer gone.Body.Close()
	if gone.StatusCode != 200 {
		t.Fatalf("delete: %d", gone.StatusCode)
	}
	if err := json.NewDecoder(gone.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if list, _ = payload["todos"].([]any); len(list) != 0 {
		t.Fatalf("left %#v", payload)
	}

	res, err = http.Post(ts.URL+"/api/todos", "application/json", strings.NewReader(
		`{"key":"`+key+`","text":"на полке","dueAt":"2026-09-14T10:00"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("add again: %d", res.StatusCode)
	}

	closeRes, err := http.Post(ts.URL+"/api/close", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer closeRes.Body.Close()
	if err := json.NewDecoder(closeRes.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("closed")
	}
	todoBook, _ = state["todoBook"].(map[string]any)
	if todoBook["key"] != key {
		t.Fatalf("last book %#v", todoBook)
	}
	if list, _ = state["todos"].([]any); len(list) != 1 {
		t.Fatalf("last todos %#v", state["todos"])
	}

	got, err := http.Get(ts.URL + "/api/todos?key=" + url.QueryEscape(key))
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	if err := json.NewDecoder(got.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if list, _ = payload["todos"].([]any); len(list) != 1 {
		t.Fatalf("get %#v", payload)
	}
}

func TestHistoryAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if hist, _ := state["history"].([]any); len(hist) != 0 {
		t.Fatalf("open should not write history %#v", hist)
	}
	bookMap, _ := state["book"].(map[string]any)
	key, _ := bookMap["key"].(string)
	if key == "" {
		t.Fatal("book key")
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/progress", strings.NewReader(`{"chapterIndex":1,"scrollRatio":0.5}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	out, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out.Body.Close()
	if out.StatusCode != 200 {
		t.Fatalf("progress: %d", out.StatusCode)
	}

	res, err = http.Post(ts.URL+"/api/history", "application/json", strings.NewReader(
		`{"text":"интересная глава","chapterIndex":1,"scrollRatio":0.5}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("note: %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	list, _ := payload["history"].([]any)
	if len(list) != 1 {
		t.Fatalf("after note %#v", payload)
	}
	item, _ := list[0].(map[string]any)
	if item["kind"] != "note" || item["text"] != "интересная глава" || item["bookKey"] != key {
		t.Fatalf("note item %#v", item)
	}

	closeRes, err := http.Post(ts.URL+"/api/close", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer closeRes.Body.Close()
	if err := json.NewDecoder(closeRes.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("closed")
	}
	hist, _ := state["history"].([]any)
	if len(hist) != 2 {
		t.Fatalf("close should add session %#v", hist)
	}
	session, _ := hist[0].(map[string]any)
	if session["kind"] != "session" || session["bookKey"] != key {
		t.Fatalf("session %#v", session)
	}
	if int(session["chapterIndex"].(float64)) != 1 {
		t.Fatalf("chapter %#v", session)
	}

	again, err := http.Post(ts.URL+"/api/history/session", "application/json", strings.NewReader(`{"chapterIndex":1,"scrollRatio":0.5}`))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Body.Close()
	if err := json.NewDecoder(again.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if list, _ = payload["history"].([]any); len(list) != 2 {
		t.Fatalf("no book session %#v", payload)
	}

	res, err = http.Post(ts.URL+"/api/history", "application/json", strings.NewReader(`{"text":"ещё мысль"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if list, _ = payload["history"].([]any); len(list) != 3 {
		t.Fatalf("shelf note %#v", payload)
	}
	if list[0].(map[string]any)["kind"] != "note" || list[0].(map[string]any)["text"] != "ещё мысль" {
		t.Fatalf("latest %#v", list[0])
	}

	empty, err := http.Post(ts.URL+"/api/history", "application/json", strings.NewReader(`{"text":""}`))
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Body.Close()
	if empty.StatusCode != 400 {
		t.Fatalf("empty note: %d", empty.StatusCode)
	}

	got, err := http.Get(ts.URL + "/api/history")
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	if err := json.NewDecoder(got.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if list, _ = payload["history"].([]any); len(list) != 3 {
		t.Fatalf("get %#v", payload)
	}
}

func TestUndoAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/bookmarks", "application/json", strings.NewReader(`{"chapterIndex":1,"scrollRatio":0.2,"title":"Место"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var added map[string]any
	if err := json.NewDecoder(res.Body).Decode(&added); err != nil {
		t.Fatal(err)
	}
	mark, _ := added["bookmark"].(map[string]any)
	id, _ := mark["id"].(string)
	if id == "" {
		t.Fatalf("bookmark %#v", added)
	}

	del, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/bookmarks?id="+url.QueryEscape(id), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("delete bookmark: %d", res.StatusCode)
	}
	var deleted map[string]any
	if err := json.NewDecoder(res.Body).Decode(&deleted); err != nil {
		t.Fatal(err)
	}
	if marks, _ := deleted["bookmarks"].([]any); len(marks) != 0 {
		t.Fatalf("still there %#v", deleted)
	}
	undo, _ := deleted["undo"].([]any)
	if len(undo) != 1 {
		t.Fatalf("undo after delete %#v", deleted)
	}
	entry, _ := undo[0].(map[string]any)
	if entry["kind"] != "delete_bookmark" {
		t.Fatalf("kind %#v", entry)
	}

	res, err = http.Get(ts.URL + "/api/undo")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var log map[string]any
	if err := json.NewDecoder(res.Body).Decode(&log); err != nil {
		t.Fatal(err)
	}
	if list, _ := log["undo"].([]any); len(list) != 1 {
		t.Fatalf("get undo %#v", log)
	}

	res, err = http.Post(ts.URL+"/api/undo", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("undo last: %d", res.StatusCode)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if marks, _ := state["bookmarks"].([]any); len(marks) != 1 {
		t.Fatalf("restored %#v", state["bookmarks"])
	}
	if list, _ := state["undo"].([]any); len(list) != 0 {
		t.Fatalf("undo after restore %#v", state["undo"])
	}

	noteRes, err := http.Post(ts.URL+"/api/notes", "application/json", strings.NewReader(
		`{"chapterIndex":1,"start":4,"end":18,"color":"blue","text":"фрагмент","body":"заметка"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	defer noteRes.Body.Close()
	var notes map[string]any
	if err := json.NewDecoder(noteRes.Body).Decode(&notes); err != nil {
		t.Fatal(err)
	}
	note, _ := notes["note"].(map[string]any)
	noteID, _ := note["id"].(string)
	if noteID == "" {
		t.Fatalf("note %#v", notes)
	}
	delNote, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/notes?id="+url.QueryEscape(noteID), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(delNote)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	olderID := ""
	if err := json.NewDecoder(res.Body).Decode(&deleted); err != nil {
		t.Fatal(err)
	}
	if list, _ := deleted["undo"].([]any); len(list) == 1 {
		olderID = list[0].(map[string]any)["id"].(string)
	}
	if olderID == "" {
		t.Fatalf("note undo %#v", deleted)
	}

	del, err = http.NewRequest(http.MethodDelete, ts.URL+"/api/bookmarks?id="+url.QueryEscape(id), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	restore, err := http.Post(ts.URL+"/api/undo/restore", "application/json", strings.NewReader(`{"id":"`+olderID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer restore.Body.Close()
	if restore.StatusCode != 200 {
		t.Fatalf("restore to: %d", restore.StatusCode)
	}
	if err := json.NewDecoder(restore.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if marks, _ := state["bookmarks"].([]any); len(marks) != 1 {
		t.Fatalf("restore bookmarks %#v", state["bookmarks"])
	}
	if nlist, _ := state["notes"].([]any); len(nlist) != 1 {
		t.Fatalf("restore notes %#v", state["notes"])
	}
	if list, _ := state["undo"].([]any); len(list) != 0 {
		t.Fatalf("restore leftover %#v", state["undo"])
	}

	missing, err := http.Post(ts.URL+"/api/undo", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != 404 {
		t.Fatalf("empty undo: %d", missing.StatusCode)
	}
}

func TestDictionaryAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "ru-en.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("книга\tbook\nтекст\ttext\nboo\tбу\n")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/dictionaries", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("upload: %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	dicts, _ := payload["dictionaries"].([]any)
	if len(dicts) != 1 || payload["dictionaryId"] == "" {
		t.Fatalf("uploaded %#v", payload)
	}

	res, err = http.Get(ts.URL + "/api/dictionaries/lookup?q=" + url.QueryEscape("книге"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var found map[string]any
	if err := json.NewDecoder(res.Body).Decode(&found); err != nil {
		t.Fatal(err)
	}
	if found["found"] != true {
		t.Fatalf("lookup %#v", found)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/library/dictionary", strings.NewReader(`{"id":""}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	off, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer off.Body.Close()
	if err := json.NewDecoder(off.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["dictionaryId"] != "" {
		t.Fatalf("deactivate %#v", payload)
	}

	res, err = http.Get(ts.URL + "/api/dictionaries/lookup?q=книга")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&found); err != nil {
		t.Fatal(err)
	}
	if found["found"] != false || found["active"] != false {
		t.Fatalf("inactive lookup %#v", found)
	}

	item, _ := dicts[0].(map[string]any)
	id, _ := item["id"].(string)
	del, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/dictionaries?id="+url.QueryEscape(id), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
}

func TestExportImportAPI(t *testing.T) {
	src := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(src, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/bookmarks", "application/json", strings.NewReader(
		`{"chapterIndex":1,"scrollRatio":0.3,"title":"Место"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("bookmark: %d", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("export: %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "zip") {
		t.Fatalf("type %s", res.Header.Get("Content-Type"))
	}
	archive, err := io.ReadAll(res.Body)
	if err != nil || len(archive) == 0 {
		t.Fatalf("archive %d %v", len(archive), err)
	}

	dst := testStore(t)
	if err := dst.Remember(store.Entry{Key: "id:old", Title: "Старая"}); err != nil {
		t.Fatal(err)
	}
	srv2 := New(dst, fs.FS(ui), nil)
	ts2 := httptest.NewServer(srv2.Handler())
	t.Cleanup(ts2.Close)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "boo-library.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	res, err = http.Post(ts2.URL+"/api/import", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("import: %d %s", res.StatusCode, body)
	}
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state["book"] != nil {
		t.Fatal("import should close book")
	}
	lib, _ := state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("library %#v", lib)
	}
	item, _ := lib[0].(map[string]any)
	if item["title"] == "" {
		t.Fatalf("imported book %#v", item)
	}
	if _, ok := dst.Entry("id:old"); ok {
		t.Fatal("old book should be replaced")
	}
	marks := dst.Bookmarks(item["key"].(string))
	if len(marks) != 1 || marks[0].Title != "Место" {
		t.Fatalf("bookmarks %#v", marks)
	}
}

func TestLibrarySearchAPI(t *testing.T) {
	st := testStore(t)
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}
	book, err := demoBook()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	srv := New(st, fs.FS(ui), book)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	ws, _ := state["workspace"].(map[string]any)
	firstID, _ := ws["id"].(string)
	firstName, _ := ws["name"].(string)
	lib, _ := state["library"].([]any)
	if len(lib) != 1 {
		t.Fatalf("library %#v", lib)
	}
	item, _ := lib[0].(map[string]any)
	key, _ := item["key"].(string)
	title, _ := item["title"].(string)
	if firstID == "" || key == "" || title == "" {
		t.Fatalf("ids %#v item %#v", ws, item)
	}

	res, err = http.Post(ts.URL+"/api/lists", "application/json", strings.NewReader(`{"name":"На отпуск","key":"`+key+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("list: %d", res.StatusCode)
	}

	res, err = http.Post(ts.URL+"/api/workspaces", "application/json", strings.NewReader(`{"name":"Учёба"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("workspace: %d", res.StatusCode)
	}

	q := string([]rune(title)[:4])
	res, err = http.Get(ts.URL + "/api/library/search?q=" + url.QueryEscape(q))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("search: %d", res.StatusCode)
	}
	var found map[string]any
	if err := json.NewDecoder(res.Body).Decode(&found); err != nil {
		t.Fatal(err)
	}
	hits, _ := found["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("hits %#v", found)
	}
	hit, _ := hits[0].(map[string]any)
	if hit["key"] != key || hit["title"] != title {
		t.Fatalf("book %#v", hit)
	}
	hitWS, _ := hit["workspace"].(map[string]any)
	if hitWS["id"] != firstID || hitWS["name"] != firstName {
		t.Fatalf("workspace %#v", hitWS)
	}
	lists, _ := hit["lists"].([]any)
	if len(lists) != 1 {
		t.Fatalf("lists %#v", hit["lists"])
	}
	list, _ := lists[0].(map[string]any)
	if list["name"] != "На отпуск" {
		t.Fatalf("list %#v", list)
	}

	res, err = http.Get(ts.URL + "/api/library/search?q=" + url.QueryEscape("неттакойкниги"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&found); err != nil {
		t.Fatal(err)
	}
	hits, _ = found["hits"].([]any)
	if len(hits) != 0 {
		t.Fatalf("empty %#v", found)
	}
}
