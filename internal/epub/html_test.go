package epub

import (
	"strings"
	"testing"
)

func TestSanitizeDataTable(t *testing.T) {
	html := sanitizeChapter([]byte(`<html><body>
<table class="note" align="center" style="color:red" onclick="alert(1)">
  <caption>Годы</caption>
  <thead>
    <tr>
      <th scope="col">Век</th>
      <th align="right">Страниц</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td valign="top">XIX</td>
      <td align="right">240</td>
    </tr>
  </tbody>
</table>
</body></html>`), "OEBPS/ch1.xhtml")

	if strings.Contains(html, "onclick") || strings.Contains(html, "style=") || strings.Contains(html, "alert") {
		t.Fatalf("unsafe attr leaked: %s", html)
	}
	if !strings.Contains(html, `class="table-wrap"`) {
		t.Fatalf("missing wrap: %s", html)
	}
	if !strings.Contains(html, "data-table") {
		t.Fatalf("missing data-table: %s", html)
	}
	if !strings.Contains(html, `scope="col"`) {
		t.Fatalf("scope dropped: %s", html)
	}
	if !strings.Contains(html, "align-right") {
		t.Fatalf("align not mapped: %s", html)
	}
	if !strings.Contains(html, "valign-top") {
		t.Fatalf("valign not mapped: %s", html)
	}
	if strings.Contains(html, `align="`) || strings.Contains(html, `valign="`) {
		t.Fatalf("raw align kept: %s", html)
	}
}

func TestSanitizeLayoutTable(t *testing.T) {
	html := sanitizeChapter([]byte(`<html><body>
<table><tr><td>левая колонка</td><td>правая колонка</td></tr></table>
</body></html>`), "OEBPS/ch1.xhtml")
	if strings.Contains(html, "table-wrap") || strings.Contains(html, "data-table") {
		t.Fatalf("layout table treated as data: %s", html)
	}
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "левая колонка") {
		t.Fatalf("layout table dropped: %s", html)
	}
}
