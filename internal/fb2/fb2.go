package fb2

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"boo/internal/epub"
	"golang.org/x/net/html/charset"
)

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
	if looksZip(data) {
		return OpenZipBytes(origin, data)
	}
	return parse(origin, data)
}

func OpenZipBytes(origin string, data []byte) (*epub.Book, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("fb2.zip: %w", err)
	}
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(path.Ext(zf.Name), ".fb2") {
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			raw, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			book, err := parse(origin, raw)
			if err != nil {
				return nil, err
			}
			book.Path = origin
			return book, nil
		}
	}
	return nil, fmt.Errorf("в архиве нет FB2")
}

func looksZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && (data[2] == 3 || data[2] == 5 || data[2] == 7)
}

func parse(origin string, data []byte) (*epub.Book, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel

	book := epub.NewMemory(origin, "fb2")
	var bodies []*xnode
	var bodyNames []string
	coverID := ""
	ids := map[string]string{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fb2: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(se.Name.Local) {
		case "title-info":
			info, err := readElem(dec, se)
			if err != nil {
				return nil, err
			}
			book.Title = firstText(info, "book-title")
			book.Language = firstText(info, "lang")
			book.Author = authorsOf(info)
			if cover := find(info, "coverpage"); cover != nil {
				if img := find(cover, "image"); img != nil {
					coverID = strings.TrimPrefix(hrefOf(img), "#")
				}
			}
		case "document-info":
			info, err := readElem(dec, se)
			if err != nil {
				return nil, err
			}
			if book.Identifier == "" {
				book.Identifier = firstText(info, "id")
			}
		case "body":
			name := attr(se, "name")
			node, err := readElem(dec, se)
			if err != nil {
				return nil, err
			}
			bodies = append(bodies, node)
			bodyNames = append(bodyNames, name)
		case "binary":
			id := attr(se, "id")
			ctype := attr(se, "content-type")
			raw, err := readTextElem(dec)
			if err != nil {
				return nil, err
			}
			bin, err := decodeBinary(raw)
			if err != nil || id == "" {
				continue
			}
			name := binName(id)
			book.Put(name, ctype, bin)
			ids["#"+id] = name
			ids[id] = name
		}
	}

	if coverID != "" {
		if name, ok := ids[coverID]; ok {
			book.CoverHref = name
		} else {
			book.CoverHref = binName(coverID)
		}
	}

	var toc []epub.TOCItem
	var raw []rawChapter
	n := 0
	noteIDs := map[string]string{}
	addBody := func(body *xnode, fallback string) {
		title := sectionTitle(body)
		if title == "" {
			title = fallback
		}
		item, chapters := walkSection(body, title, &n, noteIDs)
		if len(chapters) == 0 {
			return
		}
		raw = append(raw, chapters...)
		if len(item.Children) > 0 {
			toc = append(toc, item.Children...)
		} else {
			toc = append(toc, item)
		}
	}

	mainAdded := false
	for i, body := range bodies {
		name := strings.ToLower(bodyNames[i])
		switch name {
		case "notes", "footnotes":
			addBody(body, "Примечания")
		case "comments":
			addBody(body, "Комментарии")
		default:
			if name == "annotation" {
				addBody(body, "Аннотация")
				continue
			}
			fallback := "Книга"
			if book.Title != "" && !mainAdded {
				fallback = book.Title
			}
			addBody(body, fallback)
			mainAdded = true
		}
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("в FB2 нет текста")
	}
	for _, ch := range raw {
		html := renderNodes(ch.content, ids, noteIDs)
		if ch.title != "" && !strings.Contains(html, "<h1") {
			html = "<h1>" + escape(ch.title) + "</h1>\n" + html
		}
		book.AddChapter(ch.id, ch.href, ch.title, wrapXHTML(ch.title, html))
	}
	book.SetTOC(toc)
	book.Finish(int64(len(data)))
	return book, nil
}

type rawChapter struct {
	id      string
	href    string
	title   string
	content []*xnode
}

func walkSection(n *xnode, inherited string, seq *int, noteIDs map[string]string) (epub.TOCItem, []rawChapter) {
	title := sectionTitle(n)
	if title == "" {
		title = inherited
	}
	var nested, content []*xnode
	for _, kid := range n.kids {
		if kid.kind == 'e' && kid.local == "section" {
			nested = append(nested, kid)
		} else if kid.kind == 'e' && kid.local != "title" {
			content = append(content, kid)
		}
	}

	newChapter := func(title string, content []*xnode, section *xnode) rawChapter {
		*seq++
		href := fmt.Sprintf("c%04d.xhtml", *seq)
		if section != nil {
			if id := section.attrVal("id"); id != "" {
				noteIDs[id] = href
			}
		}
		collectIDsList(content, href, noteIDs)
		return rawChapter{id: fmt.Sprintf("c%d", *seq), href: href, title: title, content: content}
	}

	if len(nested) == 0 {
		if !hasText(content) && title == "" {
			return epub.TOCItem{ChapterIndex: -1}, nil
		}
		ch := newChapter(title, content, n)
		return epub.TOCItem{Title: title, Href: ch.href}, []rawChapter{ch}
	}

	var chapters []rawChapter
	var children []epub.TOCItem
	if hasText(content) {
		ch := newChapter(title, content, n)
		chapters = append(chapters, ch)
		children = append(children, epub.TOCItem{Title: title, Href: ch.href})
	}
	for _, kid := range nested {
		item, chs := walkSection(kid, title, seq, noteIDs)
		if len(chs) > 0 {
			children = append(children, item)
			chapters = append(chapters, chs...)
		}
	}
	href := ""
	if len(chapters) > 0 {
		href = chapters[0].href
	}
	return epub.TOCItem{Title: title, Href: href, Children: children}, chapters
}

func collectIDs(n *xnode, href string, dest map[string]string) {
	if n == nil {
		return
	}
	if id := n.attrVal("id"); id != "" {
		dest[id] = href
	}
	for _, kid := range n.kids {
		collectIDs(kid, href, dest)
	}
}

func collectIDsList(nodes []*xnode, href string, dest map[string]string) {
	for _, n := range nodes {
		collectIDs(n, href, dest)
	}
}

func renderNodes(nodes []*xnode, bins, notes map[string]string) string {
	var b strings.Builder
	for _, n := range nodes {
		writeNode(&b, n, bins, notes)
	}
	return b.String()
}

func writeNode(b *strings.Builder, n *xnode, bins, notes map[string]string) {
	if n.kind == 't' {
		b.WriteString(escape(n.text))
		return
	}
	switch n.local {
	case "p":
		fmt.Fprintf(b, "<p%s>", idAttr(n))
		writeKids(b, n, bins, notes)
		b.WriteString("</p>\n")
	case "empty-line":
		b.WriteString("<p></p>\n")
	case "subtitle":
		b.WriteString("<h3>")
		writeKids(b, n, bins, notes)
		b.WriteString("</h3>\n")
	case "title":
		b.WriteString("<h2>")
		writeKids(b, n, bins, notes)
		b.WriteString("</h2>\n")
	case "emphasis":
		b.WriteString("<em>")
		writeKids(b, n, bins, notes)
		b.WriteString("</em>")
	case "strong":
		b.WriteString("<strong>")
		writeKids(b, n, bins, notes)
		b.WriteString("</strong>")
	case "strikethrough":
		b.WriteString("<s>")
		writeKids(b, n, bins, notes)
		b.WriteString("</s>")
	case "sub":
		b.WriteString("<sub>")
		writeKids(b, n, bins, notes)
		b.WriteString("</sub>")
	case "sup":
		b.WriteString("<sup>")
		writeKids(b, n, bins, notes)
		b.WriteString("</sup>")
	case "code":
		b.WriteString("<code>")
		writeKids(b, n, bins, notes)
		b.WriteString("</code>")
	case "a":
		href := hrefOf(n)
		if strings.HasPrefix(href, "#") {
			id := strings.TrimPrefix(href, "#")
			if dest, ok := notes[id]; ok {
				fmt.Fprintf(b, `<a href="%s#%s">`, escape(dest), escape(id))
			} else {
				fmt.Fprintf(b, `<a href="#%s">`, escape(id))
			}
		} else if href != "" {
			fmt.Fprintf(b, `<a href="%s">`, escape(href))
		} else {
			b.WriteString("<span>")
			writeKids(b, n, bins, notes)
			b.WriteString("</span>")
			return
		}
		writeKids(b, n, bins, notes)
		b.WriteString("</a>")
	case "image":
		src := hrefOf(n)
		src = strings.TrimPrefix(src, "#")
		if mapped, ok := bins[src]; ok {
			src = mapped
		} else if src != "" {
			src = binName(src)
		}
		if src != "" {
			fmt.Fprintf(b, `<img src="%s" alt=""/>`, escape(src))
		}
	case "epigraph", "cite", "text-author":
		tag := "blockquote"
		if n.local == "text-author" {
			tag = "p"
		}
		fmt.Fprintf(b, "<%s>", tag)
		writeKids(b, n, bins, notes)
		fmt.Fprintf(b, "</%s>\n", tag)
	case "poem", "stanza":
		b.WriteString(`<div class="poem">`)
		writeKids(b, n, bins, notes)
		b.WriteString("</div>\n")
	case "v":
		b.WriteString("<p>")
		writeKids(b, n, bins, notes)
		b.WriteString("</p>\n")
	case "annotation":
		b.WriteString("<blockquote>")
		writeKids(b, n, bins, notes)
		b.WriteString("</blockquote>\n")
	case "table":
		b.WriteString("<table>")
		writeKids(b, n, bins, notes)
		b.WriteString("</table>\n")
	case "tr":
		b.WriteString("<tr>")
		writeKids(b, n, bins, notes)
		b.WriteString("</tr>")
	case "td", "th":
		fmt.Fprintf(b, "<%s>", n.local)
		writeKids(b, n, bins, notes)
		fmt.Fprintf(b, "</%s>", n.local)
	default:
		writeKids(b, n, bins, notes)
	}
}

func writeKids(b *strings.Builder, n *xnode, bins, notes map[string]string) {
	for _, kid := range n.kids {
		writeNode(b, kid, bins, notes)
	}
}

func idAttr(n *xnode) string {
	if id := n.attrVal("id"); id != "" {
		return ` id="` + escape(id) + `"`
	}
	return ""
}

func wrapXHTML(title, body string) []byte {
	if title == "" {
		title = "chapter"
	}
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>` + escape(title) +
		`</title></head><body>` + body + `</body></html>`)
}

func binName(id string) string {
	id = strings.TrimPrefix(id, "#")
	id = strings.ReplaceAll(id, "\\", "_")
	id = strings.ReplaceAll(id, "/", "_")
	if id == "" {
		return "bin/file"
	}
	return "bin/" + id
}

func decodeBinary(raw string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, raw)
	return base64.StdEncoding.DecodeString(clean)
}

func authorsOf(info *xnode) string {
	var names []string
	for _, a := range findAll(info, "author") {
		name := strings.TrimSpace(strings.Join(nonempty(
			firstText(a, "first-name"),
			firstText(a, "middle-name"),
			firstText(a, "last-name"),
		), " "))
		if name == "" {
			name = firstText(a, "nickname")
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func nonempty(parts ...string) []string {
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return out
}

func sectionTitle(n *xnode) string {
	if n == nil {
		return ""
	}
	if t := find(n, "title"); t != nil {
		return collapse(t.plain())
	}
	return ""
}

func hasText(nodes []*xnode) bool {
	for _, n := range nodes {
		if strings.TrimSpace(n.plain()) != "" {
			return true
		}
		if n.local == "image" {
			return true
		}
	}
	return false
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func hrefOf(n *xnode) string {
	for _, key := range []string{"href", "l:href"} {
		if v := n.attrVal(key); v != "" {
			return v
		}
	}
	for _, a := range n.attr {
		if strings.EqualFold(a.Name.Local, "href") {
			return a.Value
		}
	}
	return ""
}

func readTextElem(dec *xml.Decoder) (string, error) {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return b.String(), nil
		}
	}
}

func firstText(n *xnode, local string) string {
	if found := find(n, local); found != nil {
		return collapse(found.plain())
	}
	return ""
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func LooksLike(data []byte) bool {
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	s := string(head)
	return strings.Contains(s, "FictionBook") || strings.Contains(s, "fictionbook")
}
