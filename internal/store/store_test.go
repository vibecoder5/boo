package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRememberAndLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", Format: "fb2", ChapterN: 10}); err != nil {
		t.Fatal(err)
	}
	items := st.Library()
	if len(items) != 1 || items[0].Title != "Книга" {
		t.Fatalf("%#v", items)
	}
	path, err := st.SaveBookFile("id:1", "a.fb2", []byte("<FictionBook/>"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".fb2" {
		t.Fatalf("ext %s", path)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.Library()) != 0 {
		t.Fatal("expected empty library")
	}
}

func TestSetFinished(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFinished("id:1", true); err != os.ErrNotExist {
		t.Fatalf("missing book: %v", err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFinished("id:1", true); err != nil {
		t.Fatal(err)
	}
	e, ok := st.Entry("id:1")
	if !ok || !e.Finished || e.FinishedAt.IsZero() {
		t.Fatalf("entry %#v %v", e, ok)
	}
	p, ok := st.Progress("id:1")
	if !ok || !p.Finished || p.FinishedAt.IsZero() {
		t.Fatalf("progress %#v %v", p, ok)
	}
	if err := st.SaveProgress("id:1", Progress{Title: "Книга", Author: "А", ChapterIndex: 3, ScrollRatio: 0.4}); err != nil {
		t.Fatal(err)
	}
	p, ok = st.Progress("id:1")
	if !ok || !p.Finished || p.ChapterIndex != 3 {
		t.Fatalf("progress after save %#v %v", p, ok)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	e, ok = st.Entry("id:1")
	if !ok || !e.Finished {
		t.Fatalf("remember wiped finished %#v %v", e, ok)
	}
	if err := st.SetFinished("id:1", false); err != nil {
		t.Fatal(err)
	}
	e, _ = st.Entry("id:1")
	p, _ = st.Progress("id:1")
	if e.Finished || p.Finished || !e.FinishedAt.IsZero() || !p.FinishedAt.IsZero() {
		t.Fatalf("unset %#v %#v", e, p)
	}
}

func TestSetBookMeta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookMeta("id:1", "описание", "заметка"); err != os.ErrNotExist {
		t.Fatalf("missing book: %v", err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookMeta("id:1", "  коротко о книге  ", "большая заметка"); err != nil {
		t.Fatal(err)
	}
	e, ok := st.Entry("id:1")
	if !ok || e.Description != "коротко о книге" || e.Journal != "большая заметка" {
		t.Fatalf("entry %#v %v", e, ok)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	e, ok = st.Entry("id:1")
	if !ok || e.Description != "коротко о книге" || e.Journal != "большая заметка" {
		t.Fatalf("remember wiped meta %#v %v", e, ok)
	}
	long := strings.Repeat("я", maxBookDescription+20)
	if err := st.SetBookMeta("id:1", long, ""); err != nil {
		t.Fatal(err)
	}
	e, _ = st.Entry("id:1")
	if n := len([]rune(e.Description)); n != maxBookDescription {
		t.Fatalf("clip description %d", n)
	}
	if e.Journal != "" {
		t.Fatalf("journal should clear %#v", e)
	}
}

func TestBookmarks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	mark, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Место", ChapterIndex: 2, ScrollRatio: 0.4})
	if err != nil {
		t.Fatal(err)
	}
	if mark.ID == "" {
		t.Fatal("id")
	}
	if len(st.Bookmarks("id:1")) != 1 {
		t.Fatal("count")
	}
	dup, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Ещё", ChapterIndex: 2, ScrollRatio: 0.41})
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID != mark.ID || len(st.Bookmarks("id:1")) != 1 {
		t.Fatal("dedup")
	}
	if err := st.RemoveBookmark(mark.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Bookmarks("id:1")) != 0 {
		t.Fatal("removed")
	}
}

func TestHighlights(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	hl, err := st.AddHighlight(Highlight{
		BookKey: "id:1", ChapterIndex: 0, Start: 10, End: 20, Color: "green", Text: "фрагмент",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hl.ID == "" || hl.Color != "green" {
		t.Fatalf("%#v", hl)
	}
	if _, err := st.AddHighlight(Highlight{
		BookKey: "id:1", ChapterIndex: 0, Start: 15, End: 25, Color: "pink", Text: "перекрытие",
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Highlights("id:1")); n != 1 {
		t.Fatalf("overlap keep %d", n)
	}
	if st.Highlights("id:1")[0].Color != "pink" {
		t.Fatal("color")
	}
	if _, err := st.UpdateHighlight(st.Highlights("id:1")[0].ID, "blue"); err != nil {
		t.Fatal(err)
	}
	if st.Highlights("id:1")[0].Color != "blue" {
		t.Fatal("update")
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.Highlights("id:1")) != 0 {
		t.Fatal("removed with book")
	}
}

func TestNotes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	note, err := st.AddNote(Note{
		BookKey: "id:1", ChapterIndex: 1, Start: 4, End: 12, Color: "pink", Text: "фрагмент", Body: "мысль",
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.ID == "" || note.Color != "pink" || note.Body != "мысль" {
		t.Fatalf("%#v", note)
	}
	if _, err := st.AddNote(Note{
		BookKey: "id:1", ChapterIndex: 0, Start: 1, End: 3, Color: "blue", Text: "раньше",
	}); err != nil {
		t.Fatal(err)
	}
	list := st.Notes("id:1")
	if len(list) != 2 || list[0].ChapterIndex != 0 || list[1].ChapterIndex != 1 {
		t.Fatalf("order %#v", list)
	}
	updated, err := st.UpdateNote(note.ID, "green", "новая мысль")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Color != "green" || updated.Body != "новая мысль" {
		t.Fatalf("%#v", updated)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.Notes("id:1")) != 0 {
		t.Fatal("removed with book")
	}
}

func TestTodos(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 9, 13, 18, 0, 0, 0, time.Local)
	if _, err := st.AddTodo(Todo{BookKey: "id:1", Text: "дочитать", DueAt: due}); err != os.ErrNotExist {
		t.Fatalf("missing book: %v", err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:2", Title: "Другая"}); err != nil {
		t.Fatal(err)
	}
	first, err := st.AddTodo(Todo{BookKey: "id:1", Text: "  глава 3  ", DueAt: due})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.Text != "глава 3" || first.Done || first.BookKey != "id:1" {
		t.Fatalf("%#v", first)
	}
	later := due.Add(24 * time.Hour)
	if _, err := st.AddTodo(Todo{BookKey: "id:1", Text: "конспект", DueAt: later}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTodo(Todo{BookKey: "id:2", Text: "чужое", DueAt: due}); err != nil {
		t.Fatal(err)
	}
	list := st.Todos("id:1")
	if len(list) != 2 || list[0].Text != "глава 3" || list[1].Text != "конспект" {
		t.Fatalf("order %#v", list)
	}
	updated, err := st.UpdateTodo(first.ID, "глава 3 и 4", time.Time{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Done || updated.Text != "глава 3 и 4" || updated.DueAt.IsZero() {
		t.Fatalf("%#v", updated)
	}
	list = st.Todos("id:1")
	if len(list) != 2 || list[0].Text != "конспект" || !list[1].Done {
		t.Fatalf("done last %#v", list)
	}
	if err := st.RemoveTodo(first.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Todos("id:1")); n != 1 {
		t.Fatalf("left %d", n)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.Todos("id:1")) != 0 {
		t.Fatal("removed with book")
	}
	if len(st.Todos("id:2")) != 1 {
		t.Fatal("other book todos")
	}
	if _, err := ParseDueAt(""); err == nil {
		t.Fatal("empty due")
	}
	parsed, err := ParseDueAt("2026-09-13T18:00")
	if err != nil || parsed.Hour() != 18 {
		t.Fatalf("parse %#v %v", parsed, err)
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{Kind: HistoryNote, Text: "без книги"}); err == nil {
		t.Fatal("note without book")
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", ChapterIndex: 2, ScrollRatio: 0.4}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryNote}); err == nil {
		t.Fatal("empty note")
	}
	note, err := st.AddHistory(HistoryEntry{
		BookKey: "id:1", Kind: HistoryNote, Text: "  дочитал сцену  ", ChapterIndex: 2, ScrollRatio: 0.4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.ID == "" || note.Text != "дочитал сцену" || note.Title != "Книга" || note.Author != "А" {
		t.Fatalf("note %#v", note)
	}
	first, err := st.AddHistory(HistoryEntry{
		BookKey: "id:1", Kind: HistorySession, Title: "Книга", ChapterIndex: 1, ScrollRatio: 0.2, ChapterTitle: "Глава 2",
	})
	if err != nil {
		t.Fatal(err)
	}
	dup, err := st.AddHistory(HistoryEntry{
		BookKey: "id:1", Kind: HistorySession, Title: "Книга", ChapterIndex: 3, ScrollRatio: 0.8, ChapterTitle: "Глава 4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID != first.ID || dup.ChapterIndex != 3 || dup.ScrollRatio != 0.8 {
		t.Fatalf("dedup %#v %#v", first, dup)
	}
	if n := len(st.History(10)); n != 2 {
		t.Fatalf("after dedup %d %#v", n, st.History(10))
	}
	if st.History(10)[0].Kind != HistorySession || st.History(10)[1].Kind != HistoryNote {
		t.Fatalf("order %#v", st.History(10))
	}
	if err := st.Remember(Entry{Key: "id:2", Title: "Другая"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:2", Kind: HistorySession, Title: "Другая"}); err != nil {
		t.Fatal(err)
	}
	if n := len(st.History(10)); n != 3 {
		t.Fatalf("two books %d", n)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	left := st.History(10)
	if len(left) != 1 || left[0].BookKey != "id:2" {
		t.Fatalf("remove %#v", left)
	}
	long := strings.Repeat("я", maxHistoryText+20)
	clipped, err := st.AddHistory(HistoryEntry{BookKey: "id:2", Kind: HistoryNote, Text: long})
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(clipped.Text)); n != maxHistoryText {
		t.Fatalf("clip %d", n)
	}
	firstWS := st.CurrentWorkspace()
	if _, err := st.CreateWorkspace("Учёба"); err != nil {
		t.Fatal(err)
	}
	if len(st.History(10)) != 0 {
		t.Fatal("new workspace should not inherit history")
	}
	if err := st.SwitchWorkspace(firstWS.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(st.History(10)); n != 2 {
		t.Fatalf("back %#v", st.History(10))
	}
}

func TestSummarizeReadStats(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 9, 18, 15, 30, 0, 0, loc) // пятница
	entry := func(key string, kind string, sec int, at time.Time) HistoryEntry {
		return HistoryEntry{BookKey: key, Kind: kind, DurationSec: sec, CreatedAt: at}
	}
	entries := []HistoryEntry{
		entry("id:1", HistoryReadTime, 10, time.Date(2026, 9, 18, 10, 0, 0, 0, loc)),  // сегодня
		entry("id:1", HistoryReadTime, 20, time.Date(2026, 9, 17, 23, 0, 0, 0, loc)),  // вчера, эта неделя
		entry("id:1", HistoryReadTime, 40, time.Date(2026, 9, 14, 0, 0, 0, 0, loc)),   // понедельник
		entry("id:1", HistoryReadTime, 80, time.Date(2026, 9, 13, 23, 59, 0, 0, loc)), // воскресенье, ещё сентябрь
		entry("id:1", HistoryReadTime, 160, time.Date(2026, 8, 18, 12, 0, 0, 0, loc)), // прошлый месяц
		entry("id:2", HistoryReadTime, 1000, time.Date(2026, 9, 18, 11, 0, 0, 0, loc)),
		entry("id:1", HistoryNote, 999, time.Date(2026, 9, 18, 12, 0, 0, 0, loc)),
		entry("id:1", HistorySession, 0, time.Date(2026, 9, 18, 12, 0, 0, 0, loc)),
		entry("id:1", HistoryReadTime, 0, time.Date(2026, 9, 18, 12, 0, 0, 0, loc)),
	}
	all := SummarizeReadStats(entries, "", now)
	if all.TodaySec != 1010 || all.WeekSec != 1070 || all.MonthSec != 1150 || all.TotalSec != 1310 {
		t.Fatalf("all %#v", all)
	}
	if all.Today != "16 мин 50 с" || all.Week != "17 мин 50 с" || all.Month != "19 мин 10 с" || all.Total != "21 мин 50 с" {
		t.Fatalf("labels %#v", all)
	}
	book := SummarizeReadStats(entries, "id:1", now)
	if book.TodaySec != 10 || book.WeekSec != 70 || book.MonthSec != 150 || book.TotalSec != 310 {
		t.Fatalf("book %#v", book)
	}
	empty := SummarizeReadStats(nil, "", now)
	if empty.TodaySec != 0 || empty.Today != "0 с" {
		t.Fatalf("empty %#v", empty)
	}
}

func TestReadStatsFromHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Одна"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:2", Title: "Другая"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryReadTime, DurationSec: 12}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:2", Kind: HistoryReadTime, DurationSec: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryNote, Text: "мысль"}); err != nil {
		t.Fatal(err)
	}
	all := st.ReadStats("")
	if all.TodaySec != 20 || all.WeekSec != 20 || all.MonthSec != 20 || all.TotalSec != 20 {
		t.Fatalf("all %#v", all)
	}
	one := st.ReadStats("id:1")
	if one.TotalSec != 12 || one.Today != "12 с" {
		t.Fatalf("one %#v", one)
	}
}

func TestTrimHistoryKeepsReadTime(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryReadTime, DurationSec: 7}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxHistoryKeep; i++ {
		if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryNote, Text: fmt.Sprintf("отметка %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	list := st.History(maxHistoryKeep + 10)
	found := false
	for _, e := range list {
		if e.Kind == HistoryReadTime && e.DurationSec == 7 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("read time dropped: %d entries", len(list))
	}
	if got := st.ReadStats("id:1").TotalSec; got != 7 {
		t.Fatalf("stats %d", got)
	}
}

func TestFormatReadDuration(t *testing.T) {
	cases := []struct {
		sec  int
		want string
	}{
		{0, "0 с"},
		{1, "1 с"},
		{59, "59 с"},
		{60, "1 мин"},
		{61, "1 мин 1 с"},
		{3600, "1 ч"},
		{3601, "1 ч 1 с"},
		{3661, "1 ч 1 мин 1 с"},
		{7322, "2 ч 2 мин 2 с"},
	}
	for _, c := range cases {
		if got := FormatReadDuration(c.sec); got != c.want {
			t.Fatalf("%d: got %q want %q", c.sec, got, c.want)
		}
	}
}

func TestHistoryReadTime(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{Kind: HistoryReadTime, DurationSec: 12}); err == nil {
		t.Fatal("read time without book")
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryReadTime, DurationSec: 0}); err == nil {
		t.Fatal("zero duration")
	}
	got, err := st.AddHistory(HistoryEntry{
		BookKey: "id:1", Kind: HistoryReadTime, DurationSec: 125, ChapterIndex: 2, ScrollRatio: 0.4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != HistoryReadTime || got.DurationSec != 125 || got.Text != "Чтение: 2 мин 5 с" {
		t.Fatalf("entry %#v", got)
	}
	if got.Title != "Книга" || got.Author != "А" {
		t.Fatalf("book fill %#v", got)
	}
	capped, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryReadTime, DurationSec: maxReadDurationSec + 10})
	if err != nil {
		t.Fatal(err)
	}
	if capped.DurationSec != maxReadDurationSec {
		t.Fatalf("cap %d", capped.DurationSec)
	}
	list := st.History(10)
	if len(list) != 2 || list[0].Kind != HistoryReadTime || list[1].Kind != HistoryReadTime {
		t.Fatalf("keep both %#v", list)
	}
}

func TestWorkspacesIndependent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Workspaces()) != 1 {
		t.Fatalf("default %#v", st.Workspaces())
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Общая", ChapterIndex: 1, ScrollRatio: 0.2, ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddNote(Note{BookKey: "id:1", ChapterIndex: 1, Start: 0, End: 4, Text: "первая", Body: "из первой"}); err != nil {
		t.Fatal(err)
	}
	first := st.CurrentWorkspace()
	second, err := st.CreateWorkspace("Учёба")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Current || second.ID == first.ID {
		t.Fatalf("%#v %#v", first, second)
	}
	if len(st.Library()) != 0 {
		t.Fatal("new workspace should be empty")
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Общая", ChapterIndex: 4, ScrollRatio: 0.9, ChapterN: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddNote(Note{BookKey: "id:1", ChapterIndex: 4, Start: 2, End: 8, Text: "вторая", Body: "из второй"}); err != nil {
		t.Fatal(err)
	}
	if p, ok := st.Progress("id:1"); !ok || p.ChapterIndex != 4 {
		t.Fatalf("progress in second %#v %v", p, ok)
	}
	if n := st.Notes("id:1"); len(n) != 1 || n[0].Body != "из второй" {
		t.Fatalf("notes in second %#v", n)
	}
	if err := st.SwitchWorkspace(first.ID); err != nil {
		t.Fatal(err)
	}
	if p, ok := st.Progress("id:1"); !ok || p.ChapterIndex != 1 {
		t.Fatalf("progress in first %#v %v", p, ok)
	}
	if n := st.Notes("id:1"); len(n) != 1 || n[0].Body != "из первой" {
		t.Fatalf("notes in first %#v", n)
	}
	if st.CurrentWorkspace().ID != first.ID {
		t.Fatal("switch")
	}
}

func TestRenameWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	first := st.CurrentWorkspace()
	renamed, err := st.RenameWorkspace(first.ID, "Дом")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Дом" || !renamed.Current || renamed.ID != first.ID {
		t.Fatalf("renamed current %#v", renamed)
	}
	if st.CurrentWorkspace().Name != "Дом" {
		t.Fatalf("current %#v", st.CurrentWorkspace())
	}
	second, err := st.CreateWorkspace("Учёба")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("create should make a new workspace")
	}
	other, err := st.RenameWorkspace(first.ID, "Домашняя")
	if err != nil {
		t.Fatal(err)
	}
	if other.Name != "Домашняя" || other.Current || other.ID != first.ID {
		t.Fatalf("renamed other %#v current %#v second %#v", other, st.CurrentWorkspace(), second)
	}
	if st.CurrentWorkspace().ID != second.ID || st.CurrentWorkspace().Name != "Учёба" {
		t.Fatalf("current %#v", st.CurrentWorkspace())
	}
	list := st.Workspaces()
	names := map[string]bool{}
	for _, ws := range list {
		names[ws.Name] = true
	}
	if !names["Домашняя"] || !names["Учёба"] {
		t.Fatalf("list %#v", list)
	}
	if _, err := st.RenameWorkspace(second.ID, "   "); err == nil {
		t.Fatal("empty name")
	}
	if _, err := st.RenameWorkspace("missing", "Новое"); err == nil {
		t.Fatal("missing")
	}
}

func TestMoveBook(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{
		Key:         "id:1",
		Title:       "Общая",
		Author:      "А",
		ChapterN:    8,
		Description: "о книге",
		Journal:     "заметка",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveProgress("id:1", Progress{Title: "Общая", Author: "А", ChapterIndex: 2, ScrollRatio: 0.4}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Место", ChapterIndex: 2, ScrollRatio: 0.4}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddNote(Note{BookKey: "id:1", ChapterIndex: 2, Start: 0, End: 4, Text: "фрагмент", Body: "из первой"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHighlight(Highlight{BookKey: "id:1", ChapterIndex: 2, Start: 1, End: 3, Text: "аб"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTodo(Todo{BookKey: "id:1", Text: "дело", DueAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryNote, Text: "мысль"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTOCFold("id:1", []string{"ch-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetChapterRead("id:1", 1, true); err != nil {
		t.Fatal(err)
	}
	list, err := st.CreateList("На отпуск", "id:1")
	if err != nil {
		t.Fatal(err)
	}
	path, err := st.SaveBookFile("id:1", "a.fb2", []byte("<FictionBook/>"))
	if err != nil {
		t.Fatal(err)
	}
	first := st.CurrentWorkspace()
	second, err := st.AddWorkspace("Учёба")
	if err != nil {
		t.Fatal(err)
	}
	if second.Current || second.ID == first.ID {
		t.Fatalf("add should not switch %#v current %#v", second, first)
	}
	if st.CurrentWorkspace().ID != first.ID {
		t.Fatal("current after add")
	}
	if err := st.MoveBook("id:1", first.ID); err == nil {
		t.Fatal("same workspace")
	}
	if err := st.MoveBook("id:1", "missing"); err != os.ErrNotExist {
		t.Fatalf("missing dest: %v", err)
	}
	if err := st.MoveBook("id:missing", second.ID); err != os.ErrNotExist {
		t.Fatalf("missing book: %v", err)
	}
	if err := st.MoveBook("id:1", second.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Library()) != 0 {
		t.Fatalf("source library %#v", st.Library())
	}
	if n := st.Notes("id:1"); len(n) != 0 {
		t.Fatalf("source notes %#v", n)
	}
	if n := st.Bookmarks("id:1"); len(n) != 0 {
		t.Fatalf("source bookmarks %#v", n)
	}
	if n := st.Highlights("id:1"); len(n) != 0 {
		t.Fatalf("source highlights %#v", n)
	}
	if n := st.Todos("id:1"); len(n) != 0 {
		t.Fatalf("source todos %#v", n)
	}
	if n := len(st.History(10)); n != 0 {
		t.Fatalf("source history %d", n)
	}
	if len(st.TOCFold("id:1")) != 0 || len(st.ReadChapters("id:1")) != 0 {
		t.Fatal("source extras")
	}
	gotList := st.Lists()[0]
	if gotList.ID != list.ID || gotList.BookCount != 0 {
		t.Fatalf("list should stay empty %#v", gotList)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := st.SwitchWorkspace(second.ID); err != nil {
		t.Fatal(err)
	}
	e, ok := st.Entry("id:1")
	if !ok || e.Title != "Общая" || e.Description != "о книге" || e.Journal != "заметка" {
		t.Fatalf("dest entry %#v %v", e, ok)
	}
	p, ok := st.Progress("id:1")
	if !ok || p.ChapterIndex != 2 || p.ScrollRatio != 0.4 {
		t.Fatalf("dest progress %#v %v", p, ok)
	}
	if n := st.Notes("id:1"); len(n) != 1 || n[0].Body != "из первой" {
		t.Fatalf("dest notes %#v", n)
	}
	if n := st.Bookmarks("id:1"); len(n) != 1 || n[0].Title != "Место" {
		t.Fatalf("dest bookmarks %#v", n)
	}
	if n := st.Highlights("id:1"); len(n) != 1 || n[0].Text != "аб" {
		t.Fatalf("dest highlights %#v", n)
	}
	if n := st.Todos("id:1"); len(n) != 1 || n[0].Text != "дело" {
		t.Fatalf("dest todos %#v", n)
	}
	if n := st.History(10); len(n) != 1 || n[0].Text != "мысль" {
		t.Fatalf("dest history %#v", n)
	}
	if fold := st.TOCFold("id:1"); len(fold) != 1 || fold[0] != "ch-1" {
		t.Fatalf("dest fold %#v", fold)
	}
	if ch := st.ReadChapters("id:1"); len(ch) != 1 || ch[0] != 1 {
		t.Fatalf("dest read %#v", ch)
	}
	if err := st.SwitchWorkspace(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Общая"}); err != nil {
		t.Fatal(err)
	}
	if err := st.MoveBook("id:1", second.ID); err == nil || err.Error() != "книга уже есть в этом пространстве" {
		t.Fatalf("already: %v", err)
	}
}

func TestWorkspaceMigration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := filepath.Join(dir, "boo")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "books": {"id:1": {"title": "Старая", "chapterIndex": 2, "scrollRatio": 0.5}},
  "library": [{"key": "id:1", "title": "Старая", "chapterIndex": 2, "chapterN": 5}],
  "notes": [{"id": "n1", "bookKey": "id:1", "start": 0, "end": 2, "body": "legacy"}],
  "ui": {"theme": "dark", "fontSize": 20, "lineHeight": 1.7, "maxWidth": 38, "sidebarWidth": 280, "notesWidth": 300}
}`
	if err := os.WriteFile(filepath.Join(cfg, "state.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	list := st.Workspaces()
	if len(list) != 1 || list[0].Name != "Библиотека" || list[0].BookCount != 1 {
		t.Fatalf("%#v", list)
	}
	if p, ok := st.Progress("id:1"); !ok || p.ChapterIndex != 2 {
		t.Fatalf("progress %#v %v", p, ok)
	}
	if n := st.Notes("id:1"); len(n) != 1 || n[0].Body != "legacy" {
		t.Fatalf("notes %#v", n)
	}
}

func TestNormalizeUIFonts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	ui := st.UI()
	if ui.BookFont != "serif" || ui.UIFont != "system" || ui.BookFontSize != 20 || ui.UIFontSize != 16 || ui.FontSize != 20 {
		t.Fatalf("defaults %#v", ui)
	}
	if err := st.SetUI(UI{Theme: "light", FontSize: 24, LineHeight: 1.7, MaxWidth: 38, SidebarWidth: 280, NotesWidth: 300, HistoryWidth: 280}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if ui.BookFontSize != 24 || ui.FontSize != 24 || ui.BookFont != "serif" || ui.UIFont != "system" || ui.UIFontSize != 16 {
		t.Fatalf("legacy size %#v", ui)
	}
	if err := st.SetUI(UI{
		Theme:        "dark",
		BookFont:     "georgia",
		BookFontSize: 18,
		UIFont:       "verdana",
		UIFontSize:   14,
		LineHeight:   1.7,
		MaxWidth:     38,
		SidebarWidth: 280,
		NotesWidth:   300,
		HistoryWidth: 280,
	}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if ui.BookFont != "georgia" || ui.BookFontSize != 18 || ui.FontSize != 18 || ui.UIFont != "verdana" || ui.UIFontSize != 14 {
		t.Fatalf("custom %#v", ui)
	}
	if err := st.SetUI(UI{
		Theme:        "dark",
		BookFont:     "comic-sans",
		BookFontSize: 4,
		UIFont:       "papyrus",
		UIFontSize:   80,
		LineHeight:   1.7,
		MaxWidth:     38,
		SidebarWidth: 280,
		NotesWidth:   300,
		HistoryWidth: 280,
	}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if ui.BookFont != "serif" || ui.UIFont != "system" || ui.BookFontSize != 20 || ui.UIFontSize != 16 {
		t.Fatalf("invalid %#v", ui)
	}
}

func TestWorkspacesPanelUI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	ui := st.UI()
	if !ui.WorkspacesOpen || ui.WorkspacesWidth != 280 {
		t.Fatalf("defaults %#v", ui)
	}
	if err := st.SetUI(UI{Theme: "dark", FontSize: 20, LineHeight: 1.7, MaxWidth: 38, SidebarWidth: 280, NotesWidth: 300, HistoryWidth: 280}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if !ui.WorkspacesOpen || ui.WorkspacesWidth != 280 {
		t.Fatalf("legacy %#v", ui)
	}
	if err := st.SetUI(UI{
		Theme:           "dark",
		FontSize:        20,
		LineHeight:      1.7,
		MaxWidth:        38,
		SidebarWidth:    280,
		NotesWidth:      300,
		HistoryWidth:    280,
		WorkspacesWidth: 320,
		WorkspacesOpen:  false,
	}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if ui.WorkspacesOpen || ui.WorkspacesWidth != 320 {
		t.Fatalf("closed %#v", ui)
	}
}

func TestListsPanelUI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	ui := st.UI()
	if !ui.ListsOpen || ui.ListsWidth != 280 {
		t.Fatalf("defaults %#v", ui)
	}
	if err := st.SetUI(UI{Theme: "dark", FontSize: 20, LineHeight: 1.7, MaxWidth: 38, SidebarWidth: 280, NotesWidth: 300, HistoryWidth: 280, WorkspacesWidth: 280, WorkspacesOpen: true}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if !ui.ListsOpen || ui.ListsWidth != 280 {
		t.Fatalf("legacy %#v", ui)
	}
	if err := st.SetUI(UI{
		Theme:           "dark",
		FontSize:        20,
		LineHeight:      1.7,
		MaxWidth:        38,
		SidebarWidth:    280,
		NotesWidth:      300,
		HistoryWidth:    280,
		WorkspacesWidth: 280,
		WorkspacesOpen:  true,
		ListsWidth:      300,
		ListsOpen:       false,
	}); err != nil {
		t.Fatal(err)
	}
	ui = st.UI()
	if ui.ListsOpen || ui.ListsWidth != 300 {
		t.Fatalf("closed %#v", ui)
	}
}

func TestReadingLists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateList("", ""); err == nil {
		t.Fatal("empty name")
	}
	if _, err := st.CreateList("На отпуск", "id:missing"); err != os.ErrNotExist {
		t.Fatalf("missing book: %v", err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	list, err := st.CreateList("На отпуск", "id:1")
	if err != nil {
		t.Fatal(err)
	}
	if list.Name != "На отпуск" || list.BookCount != 1 || len(list.BookKeys) != 1 || list.BookKeys[0] != "id:1" {
		t.Fatalf("%#v", list)
	}
	if n := len(st.Lists()); n != 1 {
		t.Fatalf("count %d", n)
	}
	if err := st.Remember(Entry{Key: "id:2", Title: "Вторая"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddToList(list.ID, "id:2"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddToList(list.ID, "id:2"); err != nil {
		t.Fatal(err)
	}
	got := st.Lists()[0]
	if got.BookCount != 2 {
		t.Fatalf("after add %#v", got)
	}
	if err := st.RemoveFromList(list.ID, "id:1"); err != nil {
		t.Fatal(err)
	}
	if st.Lists()[0].BookCount != 1 || st.Lists()[0].BookKeys[0] != "id:2" {
		t.Fatalf("after remove %#v", st.Lists()[0])
	}
	if err := st.Remove("id:2"); err != nil {
		t.Fatal(err)
	}
	if st.Lists()[0].BookCount != 0 {
		t.Fatalf("book left in list %#v", st.Lists()[0])
	}
	if err := st.DeleteList(list.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Lists()) != 0 {
		t.Fatal("list left")
	}
}

func TestSetChapterRead(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetChapterRead("id:1", -1, true); err == nil {
		t.Fatal("expected bad chapter")
	}
	got, err := st.SetChapterRead("id:1", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("add %#v", got)
	}
	got, err = st.SetChapterRead("id:1", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("sorted %#v", got)
	}
	got, err = st.SetChapterRead("id:1", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("dup %#v", got)
	}
	if have := st.ReadChapters("id:1"); len(have) != 2 || have[0] != 0 || have[1] != 2 {
		t.Fatalf("get %#v", have)
	}
	got, err = st.SetChapterRead("id:1", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("unset %#v", got)
	}
	got, err = st.SetChapterRead("id:1", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty %#v", got)
	}
	if _, err := st.SetChapterRead("id:1", 1, true); err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.ReadChapters("id:1")) != 0 {
		t.Fatal("read chapters left after remove")
	}
}

func TestUndoDeletes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UndoLast(); err != os.ErrNotExist {
		t.Fatalf("empty undo: %v", err)
	}
	if n := len(st.UndoLog()); n != 0 {
		t.Fatalf("empty log %d", n)
	}
	path, err := st.SaveBookFile("id:1", "a.fb2", []byte("<FictionBook/>"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Author: "А", Path: path, Format: "fb2", ChapterN: 4}); err != nil {
		t.Fatal(err)
	}
	mark, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Место", ChapterIndex: 1, ScrollRatio: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	note, err := st.AddNote(Note{BookKey: "id:1", ChapterIndex: 1, Start: 0, End: 4, Text: "фрагмент", Body: "мысль"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHighlight(Highlight{BookKey: "id:1", ChapterIndex: 1, Start: 0, End: 4, Text: "фрагмент"}); err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 9, 13, 18, 0, 0, 0, time.Local)
	if _, err := st.AddTodo(Todo{BookKey: "id:1", Text: "дочитать", DueAt: due}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddHistory(HistoryEntry{BookKey: "id:1", Kind: HistoryNote, Text: "отметил"}); err != nil {
		t.Fatal(err)
	}
	list, err := st.CreateList("На отпуск", "id:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetChapterRead("id:1", 1, true); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveBookmark(mark.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Bookmarks("id:1")) != 0 {
		t.Fatal("bookmark still there")
	}
	log := st.UndoLog()
	if len(log) != 1 || log[0].Kind != UndoDeleteBookmark || log[0].Detail != "Место" {
		t.Fatalf("bookmark undo %#v", log)
	}
	if err := st.UndoLast(); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Bookmarks("id:1")); n != 1 || st.Bookmarks("id:1")[0].ID != mark.ID {
		t.Fatalf("bookmark restore %#v", st.Bookmarks("id:1"))
	}
	if err := st.RemoveNote(note.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Notes("id:1")) != 0 {
		t.Fatal("note still there")
	}
	if err := st.UndoLast(); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Notes("id:1")); n != 1 || st.Notes("id:1")[0].Body != "мысль" {
		t.Fatalf("note restore %#v", st.Notes("id:1"))
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.Library()) != 0 {
		t.Fatal("library")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be in trash: %v", err)
	}
	if n := len(st.UndoLog()); n != 1 || st.UndoLog()[0].Kind != UndoDeleteBook {
		t.Fatalf("book undo %#v", st.UndoLog())
	}
	if err := st.UndoLast(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file restore: %v", err)
	}
	e, ok := st.Entry("id:1")
	if !ok || e.Title != "Книга" || e.Path != path {
		t.Fatalf("entry %#v %v", e, ok)
	}
	if len(st.Bookmarks("id:1")) != 1 || len(st.Notes("id:1")) != 1 || len(st.Highlights("id:1")) != 1 {
		t.Fatalf("annos marks=%d notes=%d hl=%d", len(st.Bookmarks("id:1")), len(st.Notes("id:1")), len(st.Highlights("id:1")))
	}
	if len(st.Todos("id:1")) != 1 || len(st.History(10)) != 1 || len(st.ReadChapters("id:1")) != 1 {
		t.Fatalf("related todos=%d hist=%d read=%d", len(st.Todos("id:1")), len(st.History(10)), len(st.ReadChapters("id:1")))
	}
	if lists := st.Lists(); len(lists) != 1 || lists[0].ID != list.ID || lists[0].BookCount != 1 {
		t.Fatalf("list %#v", st.Lists())
	}
	if len(st.UndoLog()) != 0 {
		t.Fatalf("log after undo %#v", st.UndoLog())
	}
}

func TestRestoreTo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	first, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Первая", ChapterIndex: 0, ScrollRatio: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Вторая", ChapterIndex: 1, ScrollRatio: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	note, err := st.AddNote(Note{BookKey: "id:1", ChapterIndex: 0, Start: 0, End: 3, Text: "фрагмент", Body: "старая"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveBookmark(first.ID); err != nil {
		t.Fatal(err)
	}
	older := st.UndoLog()[0]
	if err := st.RemoveBookmark(second.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveNote(note.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.UndoLog()) != 3 {
		t.Fatalf("log %d", len(st.UndoLog()))
	}
	if err := st.RestoreTo("missing"); err != os.ErrNotExist {
		t.Fatalf("missing: %v", err)
	}
	if err := st.RestoreTo(older.ID); err != nil {
		t.Fatal(err)
	}
	marks := st.Bookmarks("id:1")
	if len(marks) != 2 {
		t.Fatalf("bookmarks %#v", marks)
	}
	if len(st.Notes("id:1")) != 1 || st.Notes("id:1")[0].Body != "старая" {
		t.Fatalf("notes %#v", st.Notes("id:1"))
	}
	if len(st.UndoLog()) != 0 {
		t.Fatalf("log left %#v", st.UndoLog())
	}
}

func TestUndoWorkspaceIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Домашняя"}); err != nil {
		t.Fatal(err)
	}
	mark, err := st.AddBookmark(Bookmark{BookKey: "id:1", Title: "Место", ChapterIndex: 0, ScrollRatio: 0.2})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveBookmark(mark.ID); err != nil {
		t.Fatal(err)
	}
	first := st.CurrentWorkspace()
	if _, err := st.CreateWorkspace("Учёба"); err != nil {
		t.Fatal(err)
	}
	if len(st.UndoLog()) != 0 {
		t.Fatal("new workspace inherited undo")
	}
	if err := st.SwitchWorkspace(first.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(st.UndoLog()); n != 1 || st.UndoLog()[0].Kind != UndoDeleteBookmark {
		t.Fatalf("back %#v", st.UndoLog())
	}
	if err := st.UndoLast(); err != nil {
		t.Fatal(err)
	}
	if len(st.Bookmarks("id:1")) != 1 {
		t.Fatal("restore after switch")
	}
}

func TestTOCFold(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetTOCFold("id:1", []string{"0", "1.2", "0", ""}); err != nil {
		t.Fatal(err)
	}
	got := st.TOCFold("id:1")
	if len(got) != 2 || got[0] != "0" || got[1] != "1.2" {
		t.Fatalf("%#v", got)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove("id:1"); err != nil {
		t.Fatal(err)
	}
	if len(st.TOCFold("id:1")) != 0 {
		t.Fatal("fold left after remove")
	}
}

func TestDictionaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	d, err := st.AddDictionary(Dictionary{Name: "Англо-русский", FromLang: "en", ToLang: "ru", FileName: "en-ru.txt", Ext: ".txt", WordCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || len(st.Dictionaries()) != 1 {
		t.Fatalf("%#v", d)
	}
	path, err := st.SaveDictionaryFile(d, []byte("hello\tпривет\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDictionary("id:1", d.ID); err != nil {
		t.Fatal(err)
	}
	e, ok := st.Entry("id:1")
	if !ok || e.DictionaryID != d.ID {
		t.Fatalf("active %#v %v", e, ok)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга"}); err != nil {
		t.Fatal(err)
	}
	e, _ = st.Entry("id:1")
	if e.DictionaryID != d.ID {
		t.Fatalf("remember wiped dictionary %#v", e)
	}
	if err := st.SetBookDictionary("id:1", ""); err != nil {
		t.Fatal(err)
	}
	e, _ = st.Entry("id:1")
	if e.DictionaryID != "" {
		t.Fatalf("deactivate %#v", e)
	}
	if err := st.SetBookDictionary("id:1", d.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveDictionary(d.ID); err != nil {
		t.Fatal(err)
	}
	if len(st.Dictionaries()) != 0 {
		t.Fatal("dict left")
	}
	e, _ = st.Entry("id:1")
	if e.DictionaryID != "" {
		t.Fatalf("remove should clear book %#v", e)
	}
	if _, err := st.DictionaryFile(d.ID); err != os.ErrNotExist {
		t.Fatalf("file %v", err)
	}
}

func TestSearchLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:home", Title: "Записки на полях", Author: "А"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateList("На отпуск", "id:home"); err != nil {
		t.Fatal(err)
	}
	home := st.CurrentWorkspace()
	if _, err := st.CreateWorkspace("Учёба"); err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:study", Title: "Учебник физики", Author: "Б"}); err != nil {
		t.Fatal(err)
	}

	if hits := st.SearchLibrary(""); len(hits) != 0 {
		t.Fatalf("empty %#v", hits)
	}
	if hits := st.SearchLibrary("неттакой"); len(hits) != 0 {
		t.Fatalf("miss %#v", hits)
	}

	hits := st.SearchLibrary("запис")
	if len(hits) != 1 {
		t.Fatalf("home hits %#v", hits)
	}
	if hits[0].Entry.Key != "id:home" || hits[0].WorkspaceID != home.ID || hits[0].WorkspaceName != home.Name {
		t.Fatalf("home hit %#v", hits[0])
	}
	if len(hits[0].Lists) != 1 || hits[0].Lists[0].Name != "На отпуск" {
		t.Fatalf("lists %#v", hits[0].Lists)
	}

	hits = st.SearchLibrary("УЧЕБ")
	if len(hits) != 1 || hits[0].Entry.Key != "id:study" || hits[0].WorkspaceName != "Учёба" {
		t.Fatalf("study %#v", hits)
	}
	if len(hits[0].Lists) != 0 {
		t.Fatalf("study lists %#v", hits[0].Lists)
	}

	hits = st.SearchLibrary("и")
	if len(hits) != 2 {
		t.Fatalf("both %#v", hits)
	}
	if hits[0].Entry.Key != "id:study" || !hits[0].Current {
		t.Fatalf("current first %#v", hits)
	}
}

func workspaceIDs(list []WorkspaceInfo) []string {
	out := make([]string, 0, len(list))
	for _, ws := range list {
		out = append(out, ws.ID)
	}
	return out
}

func TestWorkspaceOrderAndPin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	first := st.CurrentWorkspace()
	second, err := st.CreateWorkspace("Учёба")
	if err != nil {
		t.Fatal(err)
	}
	third, err := st.CreateWorkspace("Дача")
	if err != nil {
		t.Fatal(err)
	}
	list := st.Workspaces()
	if len(list) != 3 || list[0].ID != first.ID || list[1].ID != second.ID || list[2].ID != third.ID {
		t.Fatalf("create should keep append order, got %#v current %#v", list, st.CurrentWorkspace())
	}
	if list[2].Current != true || list[0].Current {
		t.Fatalf("current stays in place %#v", list)
	}

	if err := st.ReorderWorkspaces([]string{third.ID, first.ID, second.ID}); err != nil {
		t.Fatal(err)
	}
	list = st.Workspaces()
	if list[0].ID != third.ID || list[1].ID != first.ID || list[2].ID != second.ID {
		t.Fatalf("reorder %#v", list)
	}

	if err := st.SetWorkspacePinned(first.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetWorkspacePinned(second.ID, true); err != nil {
		t.Fatal(err)
	}
	list = st.Workspaces()
	if !list[0].Pinned || !list[1].Pinned || list[2].Pinned {
		t.Fatalf("pins %#v", list)
	}
	if list[0].ID != first.ID || list[1].ID != second.ID || list[2].ID != third.ID {
		t.Fatalf("pinned first %#v", list)
	}

	if err := st.ReorderWorkspaces([]string{second.ID, first.ID, third.ID}); err != nil {
		t.Fatal(err)
	}
	list = st.Workspaces()
	if list[0].ID != second.ID || list[1].ID != first.ID || list[2].ID != third.ID {
		t.Fatalf("pinned reorder %#v", list)
	}
	if !list[0].Pinned || !list[1].Pinned || list[2].Pinned {
		t.Fatalf("pins kept %#v", list)
	}

	st2, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	again := st2.Workspaces()
	if fmtIDs := workspaceIDs(again); len(fmtIDs) != 3 || fmtIDs[0] != second.ID || fmtIDs[1] != first.ID || fmtIDs[2] != third.ID {
		t.Fatalf("persist %#v", again)
	}
	if !again[0].Pinned || !again[1].Pinned || again[2].Pinned {
		t.Fatalf("persist pins %#v", again)
	}

	if err := st.SetWorkspacePinned(second.ID, false); err != nil {
		t.Fatal(err)
	}
	list = st.Workspaces()
	if list[0].ID != first.ID || !list[0].Pinned || list[1].Pinned || list[2].Pinned {
		t.Fatalf("unpin %#v", list)
	}

	if err := st.ReorderWorkspaces([]string{first.ID, second.ID}); err == nil {
		t.Fatal("incomplete")
	}
	if err := st.ReorderWorkspaces([]string{first.ID, second.ID, "missing"}); err == nil {
		t.Fatal("missing")
	}
	if err := st.SetWorkspacePinned("missing", true); err == nil {
		t.Fatal("pin missing")
	}
}

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestWelcomeBackground(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.WelcomeBackgroundFile(); err == nil {
		t.Fatal("expected missing background")
	}
	name, err := st.SaveWelcomeBackground(tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".png") || st.UI().WelcomeBackground != name {
		t.Fatalf("saved %#v ui %#v", name, st.UI())
	}
	path, err := st.WelcomeBackgroundFile()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, tinyPNG) {
		t.Fatalf("file %s %v", got, err)
	}
	if err := st.SetUI(UI{Theme: "light", FontSize: 22, LineHeight: 1.7, MaxWidth: 38, SidebarWidth: 280, NotesWidth: 300, HistoryWidth: 280}); err != nil {
		t.Fatal(err)
	}
	if st.UI().WelcomeBackground != name {
		t.Fatalf("setUI wiped background %#v", st.UI())
	}
	jpeg := append([]byte{0xff, 0xd8, 0xff, 0xd9}, []byte("more")...)
	next, err := st.SaveWelcomeBackground(jpeg)
	if err != nil {
		t.Fatal(err)
	}
	if next == name || !strings.HasSuffix(next, ".jpg") {
		t.Fatalf("replace %#v", next)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("old file kept %v", err)
	}
	if _, err := st.SaveWelcomeBackground([]byte("not an image")); err == nil {
		t.Fatal("expected reject")
	}
	if err := st.ClearWelcomeBackground(); err != nil {
		t.Fatal(err)
	}
	if st.UI().WelcomeBackground != "" {
		t.Fatalf("cleared %#v", st.UI())
	}
	if _, err := st.WelcomeBackgroundFile(); err == nil {
		t.Fatal("cleared file still there")
	}
}

func TestSyncFilesAndReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Persist(); err != nil {
		t.Fatal(err)
	}
	bookPath, err := st.SaveBookFile("id:1", "a.fb2", []byte("<FictionBook/>"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Remember(Entry{Key: "id:1", Title: "Книга", Path: bookPath, Format: "fb2"}); err != nil {
		t.Fatal(err)
	}
	tokenDir := filepath.Join(st.Dir(), DriveDirName)
	if err := os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "token.json"), []byte(`{"access_token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := st.SyncFiles()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.Rel] = true
		if strings.Contains(f.Rel, "token") || strings.HasPrefix(f.Rel, DriveDirName) {
			t.Fatalf("token leaked: %#v", f)
		}
	}
	if !seen["state.json"] {
		t.Fatal("missing state.json")
	}
	if len(seen) < 2 {
		t.Fatalf("expected book file too: %#v", seen)
	}

	raw, err := os.ReadFile(filepath.Join(st.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	data.Workspaces[0].Name = "После синка"
	out, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), "state.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.Reload(); err != nil {
		t.Fatal(err)
	}
	if st.CurrentWorkspace().Name != "После синка" {
		t.Fatalf("reload %#v", st.CurrentWorkspace())
	}
}

func TestSafeSyncRel(t *testing.T) {
	if _, ok := SafeSyncRel("state.json"); !ok {
		t.Fatal("state")
	}
	if _, ok := SafeSyncRel("library/book.epub"); !ok {
		t.Fatal("library")
	}
	if _, ok := SafeSyncRel("../secret"); ok {
		t.Fatal("dotdot")
	}
	if _, ok := SafeSyncRel("drive/token.json"); ok {
		t.Fatal("drive")
	}
	if _, ok := SafeSyncRel("library/foo.tmp"); ok {
		t.Fatal("tmp")
	}
}
