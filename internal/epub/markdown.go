package epub

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
)

type mdChapter struct {
	Title  string
	Subs   []mdSub
	blocks []string
}

type mdSub struct {
	Title string
	ID    string
}

type mdList struct {
	ordered bool
	items   []string
}

var (
	mdLinkRe  = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdBoldRe  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdFenceRe = regexp.MustCompile("^```")
	mdULRe    = regexp.MustCompile(`^[-*]\s+`)
	mdOLRe    = regexp.MustCompile(`^\d+\.\s+`)
	mdSepRe   = regexp.MustCompile(`^:?-+:?$`)
)

func (c *mdChapter) add(html string) {
	if html != "" {
		c.blocks = append(c.blocks, html)
	}
}

func (c *mdChapter) html() string {
	return strings.Join(c.blocks, "\n")
}

func parseMarkdown(md string) (string, []mdChapter, error) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")
	md = strings.TrimPrefix(md, "\ufeff")
	md = strings.TrimSpace(md)
	if md == "" {
		return "", nil, fmt.Errorf("пустой текст")
	}
	lines := strings.Split(md, "\n")

	title := ""
	var chapters []*mdChapter
	var cur *mdChapter
	var para []string
	var list *mdList
	inFence := false
	var fence []string

	start := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Глава"
		}
		cur = &mdChapter{Title: name}
		chapters = append(chapters, cur)
	}
	ensure := func() {
		if cur == nil {
			start("О программе")
		}
	}
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		ensure()
		cur.add("<p>" + inlineMD(strings.Join(para, " ")) + "</p>")
		para = para[:0]
	}
	flushList := func() {
		if list == nil || len(list.items) == 0 {
			return
		}
		ensure()
		tag := "ul"
		if list.ordered {
			tag = "ol"
		}
		var b strings.Builder
		b.WriteString("<" + tag + ">\n")
		for _, item := range list.items {
			b.WriteString("<li>" + inlineMD(item) + "</li>\n")
		}
		b.WriteString("</" + tag + ">")
		cur.add(b.String())
		list = nil
	}
	flushFence := func() {
		ensure()
		body := html.EscapeString(strings.Join(fence, "\n"))
		cur.add("<pre><code>" + body + "</code></pre>")
		fence = nil
	}
	flushAll := func() {
		flushPara()
		flushList()
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if inFence {
			if mdFenceRe.MatchString(strings.TrimSpace(line)) {
				flushFence()
				inFence = false
				continue
			}
			fence = append(fence, line)
			continue
		}
		if mdFenceRe.MatchString(strings.TrimSpace(line)) {
			flushAll()
			inFence = true
			fence = nil
			continue
		}

		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "# ") {
			name := strings.TrimSpace(trim[2:])
			if title == "" {
				title = name
				continue
			}
			flushAll()
			start(name)
			continue
		}
		if strings.HasPrefix(trim, "## ") {
			flushAll()
			start(strings.TrimSpace(trim[3:]))
			continue
		}
		if strings.HasPrefix(trim, "### ") {
			flushAll()
			ensure()
			name := strings.TrimSpace(trim[4:])
			id := uniqueSlug(cur, name)
			cur.Subs = append(cur.Subs, mdSub{Title: name, ID: id})
			cur.add(`<h2 id="` + html.EscapeString(id) + `">` + inlineMD(name) + `</h2>`)
			continue
		}

		if html, next, ok := parseMDTable(lines, i); ok {
			flushAll()
			ensure()
			cur.add(html)
			i = next - 1
			continue
		}

		if item, ordered, ok := parseMDListItem(trim); ok {
			flushPara()
			if list == nil || list.ordered != ordered {
				flushList()
				list = &mdList{ordered: ordered}
			}
			list.items = append(list.items, item)
			continue
		}

		if trim == "" {
			flushAll()
			continue
		}
		flushList()
		para = append(para, trim)
	}
	if inFence {
		flushFence()
	}
	flushAll()

	if title == "" {
		title = "Руководство"
	}
	if len(chapters) == 0 {
		return "", nil, fmt.Errorf("в тексте нет глав")
	}
	out := make([]mdChapter, len(chapters))
	for i, ch := range chapters {
		out[i] = *ch
	}
	return title, out, nil
}

func parseMDListItem(line string) (string, bool, bool) {
	if loc := mdULRe.FindStringIndex(line); loc != nil {
		return strings.TrimSpace(line[loc[1]:]), false, true
	}
	if loc := mdOLRe.FindStringIndex(line); loc != nil {
		return strings.TrimSpace(line[loc[1]:]), true, true
	}
	return "", false, false
}

func parseMDTable(lines []string, i int) (string, int, bool) {
	if i+1 >= len(lines) || !mdTableRow(lines[i]) || !mdTableSep(lines[i+1]) {
		return "", i, false
	}
	header := mdSplitRow(lines[i])
	aligns := mdSplitRow(lines[i+1])
	var rows [][]string
	j := i + 2
	for j < len(lines) && mdTableRow(lines[j]) && !mdTableSep(lines[j]) {
		rows = append(rows, mdSplitRow(lines[j]))
		j++
	}
	var b strings.Builder
	b.WriteString("<table>\n<thead><tr>")
	for _, cell := range header {
		b.WriteString("<th>")
		b.WriteString(inlineMD(cell))
		b.WriteString("</th>")
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range rows {
		b.WriteString("<tr>")
		for c := 0; c < len(header); c++ {
			cell := ""
			if c < len(row) {
				cell = row[c]
			}
			align := ""
			if c < len(aligns) {
				align = mdAlignClass(aligns[c])
			}
			b.WriteString("<td")
			if align != "" {
				b.WriteString(` class="` + align + `"`)
			}
			b.WriteString(">")
			b.WriteString(inlineMD(cell))
			b.WriteString("</td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody></table>")
	return b.String(), j, true
}

func mdTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "|") && !mdFenceRe.MatchString(line)
}

func mdTableSep(line string) bool {
	if !mdTableRow(line) {
		return false
	}
	for _, cell := range mdSplitRow(line) {
		cell = strings.TrimSpace(strings.ReplaceAll(cell, " ", ""))
		if !mdSepRe.MatchString(cell) {
			return false
		}
	}
	return true
}

func mdSplitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func mdAlignClass(sep string) string {
	sep = strings.TrimSpace(sep)
	left := strings.HasPrefix(sep, ":")
	right := strings.HasSuffix(sep, ":")
	switch {
	case left && right:
		return "align-center"
	case right:
		return "align-right"
	default:
		return ""
	}
}

func inlineMD(s string) string {
	var b strings.Builder
	for {
		start := strings.IndexByte(s, '`')
		if start < 0 {
			b.WriteString(inlineRich(s))
			break
		}
		rest := s[start+1:]
		end := strings.IndexByte(rest, '`')
		if end < 0 {
			b.WriteString(inlineRich(s))
			break
		}
		b.WriteString(inlineRich(s[:start]))
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(rest[:end]))
		b.WriteString("</code>")
		s = rest[end+1:]
	}
	return b.String()
}

func inlineRich(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range mdLinkRe.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(inlineBold(s[last:m[0]]))
		text := s[m[2]:m[3]]
		href := s[m[4]:m[5]]
		b.WriteString(`<a href="`)
		b.WriteString(html.EscapeString(href))
		b.WriteString(`">`)
		b.WriteString(inlineBold(text))
		b.WriteString("</a>")
		last = m[1]
	}
	b.WriteString(inlineBold(s[last:]))
	return b.String()
}

func inlineBold(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range mdBoldRe.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(html.EscapeString(s[last:m[0]]))
		b.WriteString("<strong>")
		b.WriteString(html.EscapeString(s[m[2]:m[3]]))
		b.WriteString("</strong>")
		last = m[1]
	}
	b.WriteString(html.EscapeString(s[last:]))
	return b.String()
}

func uniqueSlug(ch *mdChapter, title string) string {
	base := mdSlug(title)
	if base == "" {
		base = "section"
	}
	used := map[string]bool{}
	for _, sub := range ch.Subs {
		used[sub.ID] = true
	}
	if !used[base] {
		return base
	}
	for n := 2; ; n++ {
		id := fmt.Sprintf("%s-%d", base, n)
		if !used[id] {
			return id
		}
	}
}

func mdSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '—':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
