package txt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"boo/internal/epub"
	"golang.org/x/net/html/charset"
)

var headingRe = regexp.MustCompile(`(?m)^(?:#{1,3}\s+|(?:Глава|ГЛАВА|глава|Chapter|CHAPTER|Часть|ЧАСТЬ|часть|Part|PART)[^\n]*)$`)

func Open(path string) (*epub.Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	book, err := OpenBytes(path, data)
	if err != nil {
		return nil, err
	}
	book.Path = path
	return book, nil
}

func OpenBytes(origin string, data []byte) (*epub.Book, error) {
	text := decode(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("пустой текстовый файл")
	}

	book := epub.NewMemory(origin, formatOf(origin))
	book.Title = titleFrom(origin)
	parts := splitChapters(text)
	for i, p := range parts {
		href := fmt.Sprintf("c%04d.xhtml", i+1)
		body := "<h1>" + escape(p.title) + "</h1>\n" + paragraphs(p.body)
		book.AddChapter(fmt.Sprintf("c%d", i+1), href, p.title, wrap(p.title, body))
	}
	var toc []epub.TOCItem
	for _, ch := range book.Chapters {
		toc = append(toc, epub.TOCItem{Title: ch.Title, Href: ch.Href})
	}
	book.SetTOC(toc)
	book.Finish(int64(len(data)))
	return book, nil
}

type part struct {
	title string
	body  string
}

func splitChapters(text string) []part {
	idxs := headingRe.FindAllStringIndex(text, -1)
	if len(idxs) >= 2 {
		var parts []part
		if idxs[0][0] > 0 {
			pre := strings.TrimSpace(text[:idxs[0][0]])
			if pre != "" {
				parts = append(parts, part{title: "Начало", body: pre})
			}
		}
		for i, loc := range idxs {
			end := len(text)
			if i+1 < len(idxs) {
				end = idxs[i+1][0]
			}
			block := strings.TrimSpace(text[loc[0]:end])
			nl := strings.IndexByte(block, '\n')
			title := strings.TrimSpace(strings.TrimLeft(block, "# "))
			body := ""
			if nl >= 0 {
				title = strings.TrimSpace(strings.TrimLeft(block[:nl], "# "))
				body = strings.TrimSpace(block[nl+1:])
			}
			if title == "" {
				title = fmt.Sprintf("Глава %d", i+1)
			}
			parts = append(parts, part{title: title, body: body})
		}
		return parts
	}
	return []part{{title: "Текст", body: text}}
}

func paragraphs(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "<p></p>"
	}
	chunks := strings.Split(s, "\n")
	var b strings.Builder
	var buf []string
	flush := func() {
		if len(buf) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(escape(strings.Join(buf, " ")))
		b.WriteString("</p>\n")
		buf = buf[:0]
	}
	for _, line := range chunks {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return b.String()
}

func wrap(title, body string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>` +
		escape(title) + `</title></head><body>` + body + `</body></html>`)
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func decode(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	r, err := charset.NewReaderLabel("windows-1251", bytes.NewReader(data))
	if err != nil {
		return string(data)
	}
	out := new(bytes.Buffer)
	_, _ = out.ReadFrom(r)
	return out.String()
}

func titleFrom(origin string) string {
	base := filepath.Base(strings.ReplaceAll(origin, "\\", "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" || base == "." {
		return "Текст"
	}
	return base
}

func formatOf(origin string) string {
	switch strings.ToLower(filepath.Ext(origin)) {
	case ".md", ".markdown":
		return "md"
	default:
		return "txt"
	}
}

func LooksLike(name string, data []byte) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt", ".text", ".md", ".markdown":
		return true
	}
	if len(data) == 0 || bytes.IndexByte(data[:min(len(data), 512)], 0) >= 0 {
		return false
	}
	sample := data
	if len(sample) > 256 {
		sample = sample[:256]
	}
	if bytes.Contains(bytes.ToLower(sample), []byte("<fictionbook")) || bytes.Contains(sample, []byte("PK")) {
		return false
	}
	printable := 0
	for _, r := range string(sample) {
		if r == '\n' || r == '\r' || r == '\t' || r >= 32 {
			printable++
		}
	}
	return printable >= len([]rune(string(sample)))*8/10
}
