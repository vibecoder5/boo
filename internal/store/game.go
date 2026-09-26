package store

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	pointsVisit     = 10
	pointsOpenBook  = 5
	pointsTimer     = 5
	pointsReadTime  = 15
	pointsChapter   = 20
	dailyPagesGoal  = 5
	pageHundredths  = 100
	timerCooldown   = 2 * time.Second
	maxContinuousPg = 4.0
)

// Game — очки и ежедневные задачи профиля. Уровень считается от TotalPoints.
// MonthPoints обнуляется в новый календарный месяц, общий зачёт остаётся.
type Game struct {
	TotalPoints    int                   `json:"totalPoints"`
	MonthKey       string                `json:"monthKey,omitempty"`
	MonthPoints    int                   `json:"monthPoints"`
	AnnouncedLevel int                   `json:"announcedLevel,omitempty"`
	VisitDay       string                `json:"visitDay,omitempty"`
	Day            string                `json:"day,omitempty"`
	DailyPages     int                   `json:"dailyPages,omitempty"`
	DailyNote      bool                  `json:"dailyNote,omitempty"`
	Opened         map[string]bool       `json:"opened,omitempty"`
	Cursors        map[string]GameCursor `json:"cursors,omitempty"`
	Chapters       map[string][]int      `json:"chapters,omitempty"`
	LastTimer      time.Time             `json:"lastTimer,omitempty"`
}

type GameCursor struct {
	Chapter int     `json:"chapter"`
	Ratio   float64 `json:"ratio"`
	Screens float64 `json:"screens"`
}

type GameView struct {
	Enabled     bool        `json:"enabled"`
	Points      int         `json:"points"`
	MonthPoints int         `json:"monthPoints"`
	Level       int         `json:"level"`
	IntoLevel   int         `json:"intoLevel"`
	LevelSpan   int         `json:"levelSpan"`
	Daily       []DailyTask `json:"daily"`
	LevelUp     *LevelUp    `json:"levelUp,omitempty"`
}

type LevelUp struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type DailyTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Progress int    `json:"progress"`
	Goal     int    `json:"goal"`
	Done     bool   `json:"done"`
}

var gameNow = time.Now

func gameClock() time.Time { return gameNow() }

// PointsToReachLevel — сколько очков общего зачёта нужно, чтобы быть на уровне.
// Уровень 1 начинается с нуля, уровень n — с 25·n·(n-1).
func PointsToReachLevel(level int) int {
	if level <= 1 {
		return 0
	}
	return 25 * (level - 1) * level
}

func LevelFromPoints(points int) int {
	if points < 0 {
		points = 0
	}
	level := 1
	for PointsToReachLevel(level+1) <= points {
		level++
		if level > 10000 {
			break
		}
	}
	return level
}

func (g *Game) addPoints(n int) {
	if n <= 0 {
		return
	}
	g.TotalPoints += n
	g.MonthPoints += n
}

func (g *Game) roll(now time.Time) bool {
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")
	dirty := false
	if g.Day != day {
		g.Day = day
		g.DailyPages = 0
		g.DailyNote = false
		g.Opened = nil
		g.Cursors = nil
		dirty = true
	}
	if g.MonthKey != month {
		g.MonthKey = month
		g.MonthPoints = 0
		dirty = true
	}
	if g.TotalPoints < 0 {
		g.TotalPoints = 0
		dirty = true
	}
	if g.MonthPoints < 0 {
		g.MonthPoints = 0
		dirty = true
	}
	return dirty
}

func (s *Store) gameEnabledLocked() bool {
	return !s.data.UI.GamificationDisabled
}

func (s *Store) commitLocked(dirty bool) GameView {
	view := s.gameViewLocked()
	if dirty {
		_ = s.save()
	}
	return view
}

func (s *Store) gameViewLocked() GameView {
	g := s.data.Game
	level, into, span := levelProgress(g.TotalPoints)
	view := GameView{
		Enabled:     s.gameEnabledLocked(),
		Points:      g.TotalPoints,
		MonthPoints: g.MonthPoints,
		Level:       level,
		IntoLevel:   into,
		LevelSpan:   span,
		Daily:       dailyTasks(g),
	}
	if view.Enabled && level > 1 && level > g.AnnouncedLevel {
		view.LevelUp = &LevelUp{
			Level: level,
			Text:  fmt.Sprintf("Новый уровень: %d", level),
		}
	}
	return view
}

func levelProgress(points int) (level, into, span int) {
	level = LevelFromPoints(points)
	base := PointsToReachLevel(level)
	next := PointsToReachLevel(level + 1)
	into = points - base
	span = next - base
	if span < 1 {
		span = 1
	}
	if into < 0 {
		into = 0
	}
	return level, into, span
}

func dailyTasks(g Game) []DailyTask {
	pages := g.DailyPages
	if pages < 0 {
		pages = 0
	}
	goalPages := dailyPagesGoal * pageHundredths
	noteProgress := 0
	if g.DailyNote {
		noteProgress = 1
	}
	return []DailyTask{
		{
			ID:       "pages",
			Title:    "Прочитать 5 страниц",
			Kind:     "pages",
			Progress: pages,
			Goal:     goalPages,
			Done:     pages >= goalPages,
		},
		{
			ID:       "note",
			Title:    "Написать заметку",
			Kind:     "note",
			Progress: noteProgress,
			Goal:     1,
			Done:     g.DailyNote,
		},
	}
}

// TouchGame начисляет заход и, если книга уже открыта, открытие.
// Повтор в тот же день ту же книгу не увеличивает счёт.
func (s *Store) TouchGame(bookKey string) GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() {
		if s.awardVisitLocked() {
			dirty = true
		}
		if s.awardOpenLocked(bookKey) {
			dirty = true
		}
	}
	return s.commitLocked(dirty)
}

func (s *Store) awardVisitLocked() bool {
	day := gameClock().Format("2006-01-02")
	if s.data.Game.VisitDay == day {
		return false
	}
	s.data.Game.VisitDay = day
	s.data.Game.addPoints(pointsVisit)
	return true
}

func (s *Store) awardOpenLocked(bookKey string) bool {
	bookKey = strings.TrimSpace(bookKey)
	if bookKey == "" {
		return false
	}
	if s.data.Game.Opened == nil {
		s.data.Game.Opened = map[string]bool{}
	}
	if s.data.Game.Opened[bookKey] {
		return false
	}
	s.data.Game.Opened[bookKey] = true
	s.data.Game.addPoints(pointsOpenBook)
	return true
}

// AwardTimerStart — очки за новый запуск, не за «продолжить».
// Повтор быстрее двух секунд не считается.
func (s *Store) AwardTimerStart() GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := gameClock()
	dirty := s.data.Game.roll(now)
	if s.gameEnabledLocked() {
		if s.data.Game.LastTimer.IsZero() || now.Sub(s.data.Game.LastTimer) >= timerCooldown {
			s.data.Game.LastTimer = now
			s.data.Game.addPoints(pointsTimer)
			dirty = true
		}
	}
	return s.commitLocked(dirty)
}

// AwardReadTime — очки за сохранённую остановку таймера.
func (s *Store) AwardReadTime() GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() {
		s.data.Game.addPoints(pointsReadTime)
		dirty = true
	}
	return s.commitLocked(dirty)
}

// AwardChapter — очки за первую отметку этой главы. Снятие и повтор не платят снова.
func (s *Store) AwardChapter(bookKey string, index int) GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() && index >= 0 && strings.TrimSpace(bookKey) != "" {
		if s.data.Game.Chapters == nil {
			s.data.Game.Chapters = map[string][]int{}
		}
		list := s.data.Game.Chapters[bookKey]
		if !gameHasInt(list, index) {
			s.data.Game.Chapters[bookKey] = append(list, index)
			s.data.Game.addPoints(pointsChapter)
			dirty = true
		}
	}
	return s.commitLocked(dirty)
}

// TrackPages считает новые экраны читалки в сегодняшнюю задачу.
// screens — высота главы в экранах. Ноль не двигает курсор.
// Прыжок дальше четырёх экранов считается одним экраном.
func (s *Store) TrackPages(bookKey string, chapter int, ratio, screens float64) GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() && strings.TrimSpace(bookKey) != "" && chapter >= 0 && screens > 0 {
		if s.addPagesLocked(bookKey, chapter, ratio, screens) {
			dirty = true
		}
	}
	return s.commitLocked(dirty)
}

func (s *Store) addPagesLocked(bookKey string, chapter int, ratio, screens float64) bool {
	if screens < 0.2 {
		screens = 0.2
	}
	if screens > 400 {
		screens = 400
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	if s.data.Game.Cursors == nil {
		s.data.Game.Cursors = map[string]GameCursor{}
	}
	cur, ok := s.data.Game.Cursors[bookKey]
	add := 0.0
	next := GameCursor{Chapter: chapter, Ratio: ratio, Screens: screens}
	if !ok || cur.Chapter != chapter {
		add = 1
	} else if ratio > cur.Ratio+0.0005 {
		add = (ratio - cur.Ratio) * screens
		if add > maxContinuousPg {
			add = 1
		}
	} else {
		next.Ratio = cur.Ratio
	}
	s.data.Game.Cursors[bookKey] = next
	if add <= 0 {
		return false
	}
	hundredths := int(math.Round(add * pageHundredths))
	if hundredths < 1 {
		hundredths = 1
	}
	s.data.Game.DailyPages += hundredths
	return true
}

// NoteTextChanged — в тексте заметки появилось что-то новое.
func NoteTextChanged(prev, next string) bool {
	next = strings.TrimSpace(next)
	return next != "" && next != strings.TrimSpace(prev)
}

// RememberDailyNote отмечает сегодняшнюю заметку, не отдавая снимок клиенту.
func (s *Store) RememberDailyNote() {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() && !s.data.Game.DailyNote {
		s.data.Game.DailyNote = true
		dirty = true
	}
	if dirty {
		_ = s.save()
	}
}

// AwardDailyNote отмечает задачу, если текст заметки изменился, и отдаёт снимок.
func (s *Store) AwardDailyNote(prev, next string) GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	if s.gameEnabledLocked() && NoteTextChanged(prev, next) && !s.data.Game.DailyNote {
		s.data.Game.DailyNote = true
		dirty = true
	}
	return s.commitLocked(dirty)
}

// AckLevel запоминает, что сообщение об этом уровне уже показали.
func (s *Store) AckLevel(level int) GameView {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty := s.data.Game.roll(gameClock())
	cur := LevelFromPoints(s.data.Game.TotalPoints)
	if level > 1 && level == cur && cur > s.data.Game.AnnouncedLevel {
		s.data.Game.AnnouncedLevel = cur
		dirty = true
	}
	return s.commitLocked(dirty)
}

func gameHasInt(list []int, n int) bool {
	for _, item := range list {
		if item == n {
			return true
		}
	}
	return false
}
