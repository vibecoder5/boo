package epub

import (
	"strings"
	"testing"
)

func TestParseMarkdownBlocks(t *testing.T) {
	src := `# Заголовок

Введение про **boo** и ` + "`флаг`" + `.

## Глава

Список:

1. Первый
2. Второй с [ссылкой](https://example.com)

- Пункт

` + "```text\nкод\nстрока\n```" + `

### Подглава

| А | Б |
| --- | ---: |
| раз | 2 |
`
	title, chapters, err := parseMarkdown(src)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Заголовок" {
		t.Fatalf("title %q", title)
	}
	if len(chapters) != 2 {
		t.Fatalf("chapters %d", len(chapters))
	}
	if chapters[0].Title != "О программе" {
		t.Fatalf("intro title %q", chapters[0].Title)
	}
	if !strings.Contains(chapters[0].html(), "<strong>boo</strong>") {
		t.Fatalf("bold missing: %s", chapters[0].html())
	}
	if !strings.Contains(chapters[0].html(), "<code>флаг</code>") {
		t.Fatalf("code missing: %s", chapters[0].html())
	}
	html := chapters[1].html()
	if !strings.Contains(html, "<ol>") || !strings.Contains(html, "<ul>") {
		t.Fatalf("lists missing: %s", html)
	}
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Fatalf("link missing: %s", html)
	}
	if !strings.Contains(html, "<pre><code>") || !strings.Contains(html, "код") {
		t.Fatalf("fence missing: %s", html)
	}
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "align-right") {
		t.Fatalf("table missing: %s", html)
	}
	if len(chapters[1].Subs) != 1 || chapters[1].Subs[0].ID != "подглава" {
		t.Fatalf("subs %#v", chapters[1].Subs)
	}
	if !strings.Contains(html, `id="подглава"`) {
		t.Fatalf("sub id missing: %s", html)
	}
}

func TestParseMarkdownEmpty(t *testing.T) {
	if _, _, err := parseMarkdown("   "); err == nil {
		t.Fatal("expected error")
	}
}
