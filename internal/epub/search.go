package epub

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type Hit struct {
	ChapterIndex int    `json:"chapterIndex"`
	Title        string `json:"title"`
	Snippet      string `json:"snippet"`
	Offset       int    `json:"offset"`
}

func (b *Book) Search(query string, limit int) ([]Hit, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 2 || limit <= 0 {
		return nil, nil
	}
	if err := b.ensurePlain(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	texts := append([]string(nil), b.plain...)
	b.mu.Unlock()

	var hits []Hit
	for i, text := range texts {
		if len(hits) >= limit {
			break
		}
		offsets := findRune(text, query)
		perChapter := 0
		for _, at := range offsets {
			if len(hits) >= limit || perChapter >= 6 {
				break
			}
			hits = append(hits, Hit{
				ChapterIndex: i,
				Title:        b.Chapters[i].Title,
				Snippet:      makeSnippet(text, at, utf8.RuneCountInString(query)),
				Offset:       perChapter,
			})
			perChapter++
		}
	}
	return hits, nil
}

func (b *Book) ensurePlain() error {
	b.mu.Lock()
	if len(b.plain) == len(b.Chapters) && b.plain != nil {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	texts := make([]string, len(b.Chapters))
	for i := range b.Chapters {
		raw, err := b.ChapterHTML(i)
		if err != nil {
			texts[i] = ""
			continue
		}
		texts[i] = htmlToText(raw)
	}
	b.mu.Lock()
	b.plain = texts
	b.mu.Unlock()
	return nil
}

func htmlToText(raw string) string {
	doc, err := html.Parse(strings.NewReader("<div>" + raw + "</div>"))
	if err != nil {
		return stripTags(raw)
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "br", "li", "h1", "h2", "h3", "h4", "tr", "blockquote":
				b.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return collapseWS(b.String())
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
			b.WriteByte(' ')
		case !in:
			b.WriteRune(r)
		}
	}
	return collapseWS(b.String())
}

func collapseWS(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func findRune(text, query string) []int {
	rt := []rune(strings.ToLower(text))
	rq := []rune(strings.ToLower(query))
	if len(rq) == 0 || len(rt) < len(rq) {
		return nil
	}
	var out []int
	for i := 0; i+len(rq) <= len(rt); i++ {
		if matchAt(rt, rq, i) {
			out = append(out, i)
			i += len(rq) - 1
		}
	}
	return out
}

func matchAt(text, q []rune, i int) bool {
	for j, r := range q {
		if text[i+j] != r {
			return false
		}
	}
	return true
}

func makeSnippet(text string, at, matchRunes int) string {
	runes := []rune(text)
	if at < 0 || at >= len(runes) {
		return ""
	}
	start := at - 42
	if start < 0 {
		start = 0
	}
	end := at + matchRunes + 42
	if end > len(runes) {
		end = len(runes)
	}
	s := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		s = "…" + s
	}
	if end < len(runes) {
		s += "…"
	}
	return s
}
