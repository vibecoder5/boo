package store

import (
	"testing"
)

func TestImportWorkspaceKeepsCurrentShelf(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if st.UI().ImportWorkspace != DefaultImportWorkspace {
		t.Fatalf("default %q", st.UI().ImportWorkspace)
	}
	cur := st.CurrentWorkspace()
	info, err := st.EnsureNamedWorkspace("")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Backlog" || info.ID == cur.ID {
		t.Fatalf("created %#v current %s", info, cur.ID)
	}
	if st.CurrentWorkspace().ID != cur.ID {
		t.Fatal("switched workspace")
	}
	again, err := st.EnsureNamedWorkspace("backlog")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != info.ID {
		t.Fatalf("duplicate %s %s", again.ID, info.ID)
	}
	added, skipped, err := st.AddToWorkspace(info.ID, []Entry{{
		Key: "id:1", Title: "Книга", Author: "А", Format: "epub", ChapterN: 3,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || skipped != 0 {
		t.Fatalf("add %d %d", added, skipped)
	}
	if len(st.Library()) != 0 {
		t.Fatalf("current shelf %#v", st.Library())
	}
	if err := st.SwitchWorkspace(info.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Library()) != 1 || st.Library()[0].Title != "Книга" {
		t.Fatalf("backlog %#v", st.Library())
	}
	added, skipped, err = st.AddToWorkspace(info.ID, []Entry{{Key: "id:1", Title: "Книга"}})
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || skipped != 1 {
		t.Fatalf("second %d %d", added, skipped)
	}
	if got := normalizeUI(UI{}); got.ImportWorkspace != "Backlog" {
		t.Fatalf("empty %q", got.ImportWorkspace)
	}
	if got := normalizeUI(UI{ImportWorkspace: "  Очередь  "}); got.ImportWorkspace != "Очередь" {
		t.Fatalf("custom %q", got.ImportWorkspace)
	}
}
