package store

import (
	"testing"
	"time"
)

func setGameNow(t *testing.T, at time.Time) {
	t.Helper()
	prev := gameNow
	gameNow = func() time.Time { return at }
	t.Cleanup(func() { gameNow = prev })
}

func openGameStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestLevelFromPoints(t *testing.T) {
	cases := []struct {
		points int
		level  int
	}{
		{0, 1},
		{49, 1},
		{50, 2},
		{149, 2},
		{150, 3},
		{300, 4},
	}
	for _, c := range cases {
		if got := LevelFromPoints(c.points); got != c.level {
			t.Fatalf("points %d: level %d, want %d", c.points, got, c.level)
		}
		if PointsToReachLevel(c.level) > c.points {
			t.Fatalf("threshold %d above %d", PointsToReachLevel(c.level), c.points)
		}
	}
}

func TestGameVisitOpenAndDay(t *testing.T) {
	day := time.Date(2026, 9, 26, 8, 0, 0, 0, time.Local)
	setGameNow(t, day)
	st := openGameStore(t)

	first := st.TouchGame("")
	if !first.Enabled || first.Points != pointsVisit || first.MonthPoints != pointsVisit || first.Level != 1 {
		t.Fatalf("visit %#v", first)
	}
	again := st.TouchGame("id:book")
	if again.Points != pointsVisit+pointsOpenBook {
		t.Fatalf("open %#v", again)
	}
	same := st.TouchGame("id:book")
	if same.Points != again.Points {
		t.Fatalf("repeat open %#v", same)
	}
	other := st.TouchGame("id:other")
	if other.Points != again.Points+pointsOpenBook {
		t.Fatalf("second book %#v", other)
	}

	setGameNow(t, day.Add(24*time.Hour))
	next := st.TouchGame("id:book")
	if next.Points != other.Points+pointsVisit+pointsOpenBook {
		t.Fatalf("next day %#v", next)
	}
	if next.MonthPoints != next.Points {
		t.Fatalf("same month %#v", next)
	}
	pages := taskByID(t, next, "pages")
	if pages.Progress != 0 || pages.Done {
		t.Fatalf("daily pages reset %#v", pages)
	}
}

func TestGameMonthKeepsTotal(t *testing.T) {
	setGameNow(t, time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local))
	st := openGameStore(t)
	st.TouchGame("")
	for i := 0; i < 3; i++ {
		st.AwardReadTime()
	}
	before := st.TouchGame("")
	if before.Points != pointsVisit+3*pointsReadTime || before.MonthPoints != before.Points {
		t.Fatalf("sept %#v", before)
	}

	setGameNow(t, time.Date(2026, 10, 1, 0, 10, 0, 0, time.Local))
	rolled := st.TrackPages("", 0, 0, 0)
	if rolled.Points != before.Points || rolled.MonthPoints != 0 || rolled.Level != before.Level {
		t.Fatalf("october reset %#v", rolled)
	}
	after := st.TouchGame("")
	if after.Points != before.Points+pointsVisit || after.MonthPoints != pointsVisit {
		t.Fatalf("october visit %#v", after)
	}
}

func TestGameAwardsAndLevelUp(t *testing.T) {
	setGameNow(t, time.Date(2026, 9, 26, 9, 0, 0, 0, time.Local))
	st := openGameStore(t)

	start := st.AwardTimerStart()
	if start.Points != pointsTimer {
		t.Fatalf("timer %#v", start)
	}
	quick := st.AwardTimerStart()
	if quick.Points != start.Points {
		t.Fatalf("cooldown %#v", quick)
	}
	setGameNow(t, time.Date(2026, 9, 26, 9, 0, 3, 0, time.Local))
	later := st.AwardTimerStart()
	if later.Points != start.Points+pointsTimer {
		t.Fatalf("second start %#v", later)
	}

	read := st.AwardReadTime()
	if read.Points != later.Points+pointsReadTime {
		t.Fatalf("read time %#v", read)
	}
	ch := st.AwardChapter("id:book", 2)
	if ch.Points != read.Points+pointsChapter {
		t.Fatalf("chapter %#v", ch)
	}
	again := st.AwardChapter("id:book", 2)
	if again.Points != ch.Points {
		t.Fatalf("chapter twice %#v", again)
	}

	var view GameView
	for view.Level < 2 {
		view = st.AwardReadTime()
	}
	if view.LevelUp == nil || view.LevelUp.Level != 2 || view.LevelUp.Text != "Новый уровень: 2" {
		t.Fatalf("level up %#v", view.LevelUp)
	}
	pending := st.TouchGame("")
	if pending.LevelUp == nil || pending.LevelUp.Level != 2 {
		t.Fatalf("still pending %#v", pending.LevelUp)
	}
	acked := st.AckLevel(2)
	if acked.LevelUp != nil {
		t.Fatalf("acked %#v", acked.LevelUp)
	}
}

func TestGameDailyTasks(t *testing.T) {
	setGameNow(t, time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local))
	st := openGameStore(t)

	first := st.TrackPages("id:book", 0, 0, 4)
	pages := taskByID(t, first, "pages")
	if pages.Progress != 100 {
		t.Fatalf("open screen %#v", pages)
	}
	scrolled := st.TrackPages("id:book", 0, 0.25, 4)
	pages = taskByID(t, scrolled, "pages")
	if pages.Progress != 200 {
		t.Fatalf("scroll %#v", pages)
	}
	jump := st.TrackPages("id:book", 0, 1, 20)
	pages = taskByID(t, jump, "pages")
	if pages.Progress != 300 {
		t.Fatalf("jump %#v", pages)
	}
	stay := st.TrackPages("id:book", 0, 1, 20)
	pages = taskByID(t, stay, "pages")
	if pages.Progress != 300 {
		t.Fatalf("stay %#v", pages)
	}

	blank := st.AwardDailyNote("", "  ")
	note := taskByID(t, blank, "note")
	if note.Done {
		t.Fatal("blank note counted")
	}
	written := st.AwardDailyNote("", "мысль")
	note = taskByID(t, written, "note")
	if !note.Done || note.Progress != 1 {
		t.Fatalf("note %#v", note)
	}

	opened := st.TrackPages("id:book", 1, 0, 4)
	pages = taskByID(t, opened, "pages")
	if pages.Progress != 400 || pages.Done {
		t.Fatalf("next chapter %#v", pages)
	}
	done := st.TrackPages("id:book", 1, 0.5, 4)
	pages = taskByID(t, done, "pages")
	if pages.Progress != 600 || !pages.Done {
		t.Fatalf("five pages %#v", pages)
	}
}

func TestGameDisabledKeepsScore(t *testing.T) {
	setGameNow(t, time.Date(2026, 9, 26, 11, 0, 0, 0, time.Local))
	st := openGameStore(t)
	got := st.TouchGame("id:book")
	ui := st.UI()
	ui.GamificationDisabled = true
	if err := st.SetUI(ui); err != nil {
		t.Fatal(err)
	}
	off := st.TouchGame("id:other")
	if off.Enabled || off.Points != got.Points || off.MonthPoints != got.MonthPoints {
		t.Fatalf("disabled %#v", off)
	}
	if st.AwardTimerStart().Points != got.Points || st.AwardReadTime().Points != got.Points {
		t.Fatal("disabled awards")
	}
	if st.AwardChapter("id:book", 0).Points != got.Points {
		t.Fatal("disabled chapter")
	}
	if taskByID(t, st.AwardDailyNote("", "текст"), "note").Done {
		t.Fatal("disabled note")
	}
	if taskByID(t, st.TrackPages("id:book", 0, 0.2, 10), "pages").Progress != 0 {
		t.Fatal("disabled pages")
	}

	ui.GamificationDisabled = false
	if err := st.SetUI(ui); err != nil {
		t.Fatal(err)
	}
	on := st.TouchGame("id:book")
	if !on.Enabled || on.Points != got.Points {
		t.Fatalf("enabled again %#v", on)
	}
}

func TestGamePersists(t *testing.T) {
	setGameNow(t, time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local))
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	st, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	st.TouchGame("id:book")
	st.AwardChapter("id:book", 1)
	st2, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	view := st2.TouchGame("id:book")
	if view.Points != pointsVisit+pointsOpenBook+pointsChapter {
		t.Fatalf("reopen %#v", view)
	}
}

func taskByID(t *testing.T, view GameView, id string) DailyTask {
	t.Helper()
	for _, task := range view.Daily {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("no task %s in %#v", id, view.Daily)
	return DailyTask{}
}
