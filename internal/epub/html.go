package epub

import (
	"bytes"
	"net/url"
	"strings"

	stdhtml "html"

	"golang.org/x/net/html"
)

var allowedTags = map[string]map[string]bool{
	"p":          {"id": true, "class": true},
	"h1":         {"id": true, "class": true},
	"h2":         {"id": true, "class": true},
	"h3":         {"id": true, "class": true},
	"h4":         {"id": true, "class": true},
	"h5":         {"id": true, "class": true},
	"h6":         {"id": true, "class": true},
	"div":        {"id": true, "class": true},
	"span":       {"id": true, "class": true},
	"section":    {"id": true, "class": true},
	"article":    {"id": true, "class": true},
	"header":     {"id": true, "class": true},
	"footer":     {"id": true, "class": true},
	"blockquote": {"id": true, "class": true},
	"q":          {"id": true, "class": true},
	"cite":       {"id": true, "class": true},
	"em":         {"id": true, "class": true},
	"i":          {"id": true, "class": true},
	"strong":     {"id": true, "class": true},
	"b":          {"id": true, "class": true},
	"u":          {"id": true, "class": true},
	"s":          {"id": true, "class": true},
	"mark":       {"id": true, "class": true},
	"small":      {"id": true, "class": true},
	"sub":        {"id": true, "class": true},
	"sup":        {"id": true, "class": true},
	"ul":         {"id": true, "class": true},
	"ol":         {"id": true, "class": true},
	"li":         {"id": true, "class": true},
	"dl":         {"id": true, "class": true},
	"dt":         {"id": true, "class": true},
	"dd":         {"id": true, "class": true},
	"br":         {"class": true},
	"hr":         {"class": true},
	"a":          {"id": true, "class": true, "href": true, "title": true},
	"img":        {"id": true, "class": true, "src": true, "alt": true, "title": true, "width": true, "height": true},
	"figure":     {"id": true, "class": true},
	"figcaption": {"id": true, "class": true},
	"picture":    {"id": true, "class": true},
	"source":     {"src": true, "srcset": true, "type": true},
	"table":      {"id": true, "class": true, "align": true},
	"thead":      {"id": true, "class": true},
	"tbody":      {"id": true, "class": true},
	"tfoot":      {"id": true, "class": true},
	"tr":         {"id": true, "class": true, "align": true, "valign": true},
	"th":         {"id": true, "class": true, "colspan": true, "rowspan": true, "scope": true, "headers": true, "align": true, "valign": true},
	"td":         {"id": true, "class": true, "colspan": true, "rowspan": true, "headers": true, "align": true, "valign": true},
	"caption":    {"id": true, "class": true, "align": true},
	"colgroup":   {"span": true, "align": true},
	"col":        {"span": true, "align": true},
	"pre":        {"id": true, "class": true},
	"code":       {"id": true, "class": true},
	"abbr":       {"id": true, "class": true, "title": true},
	"svg":        {"viewbox": true, "width": true, "height": true, "xmlns": true},
	"path":       {"d": true, "fill": true, "stroke": true, "transform": true},
	"g":          {"transform": true, "fill": true, "stroke": true},
	"rect":       {"x": true, "y": true, "width": true, "height": true, "fill": true, "stroke": true, "rx": true, "ry": true},
	"circle":     {"cx": true, "cy": true, "r": true, "fill": true, "stroke": true},
	"ellipse":    {"cx": true, "cy": true, "rx": true, "ry": true, "fill": true, "stroke": true},
	"line":       {"x1": true, "y1": true, "x2": true, "y2": true, "stroke": true},
	"polyline":   {"points": true, "fill": true, "stroke": true},
	"polygon":    {"points": true, "fill": true, "stroke": true},
	"image":      {"href": true, "x": true, "y": true, "width": true, "height": true},
	"text":       {"x": true, "y": true, "fill": true},
	"tspan":      {"x": true, "y": true},
}

var dropTags = map[string]bool{
	"script":        true,
	"style":         true,
	"iframe":        true,
	"object":        true,
	"embed":         true,
	"form":          true,
	"input":         true,
	"button":        true,
	"textarea":      true,
	"select":        true,
	"link":          true,
	"meta":          true,
	"base":          true,
	"noscript":      true,
	"foreignobject": true,
}

func sanitizeChapter(data []byte, chapterHref string) string {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return stdhtml.EscapeString(string(data))
	}
	body := findChild(doc, "body")
	if body == nil {
		body = doc
	}
	out := &html.Node{Type: html.ElementNode, Data: "div"}
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		appendSanitized(out, c, chapterHref)
	}
	var buf bytes.Buffer
	for c := out.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&buf, c)
	}
	return buf.String()
}

func appendSanitized(parent, n *html.Node, chapterHref string) {
	switch n.Type {
	case html.TextNode:
		parent.AppendChild(&html.Node{Type: html.TextNode, Data: n.Data})
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		if dropTags[tag] {
			return
		}
		allowed, ok := allowedTags[tag]
		if !ok {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				appendSanitized(parent, c, chapterHref)
			}
			return
		}
		node := &html.Node{Type: html.ElementNode, Data: tag}
		node.Attr = filterAttrs(n, tag, allowed, chapterHref)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			appendSanitized(node, c, chapterHref)
		}
		if tag == "table" && tableLooksLikeData(n) {
			node.Attr = appendClass(node.Attr, "data-table")
			wrap := &html.Node{
				Type: html.ElementNode,
				Data: "div",
				Attr: []html.Attribute{{Key: "class", Val: "table-wrap"}},
			}
			wrap.AppendChild(node)
			parent.AppendChild(wrap)
			return
		}
		parent.AppendChild(node)
	}
}

func filterAttrs(n *html.Node, tag string, allowed map[string]bool, chapterHref string) []html.Attribute {
	var out []html.Attribute
	for _, a := range n.Attr {
		key := strings.ToLower(a.Key)
		if strings.HasPrefix(key, "on") {
			continue
		}
		if key == "xlink:href" {
			key = "href"
		}
		if !allowed[key] {
			continue
		}
		val := strings.TrimSpace(a.Val)
		switch key {
		case "href":
			if tag == "a" {
				out = append(out, rewriteAnchor(val, chapterHref)...)
				continue
			}
			if rewritten, ok := rewriteResource(val, chapterHref); ok {
				out = append(out, html.Attribute{Key: "href", Val: rewritten})
			}
		case "src":
			if rewritten, ok := rewriteResource(val, chapterHref); ok {
				out = append(out, html.Attribute{Key: "src", Val: rewritten})
			}
		case "srcset":
			continue
		case "align":
			if cls := tableAlignClass(val); cls != "" {
				out = appendClass(out, cls)
			}
		case "valign":
			if cls := tableValignClass(val); cls != "" {
				out = appendClass(out, cls)
			}
		default:
			out = append(out, html.Attribute{Key: key, Val: val})
		}
	}
	return out
}

func tableLooksLikeData(n *html.Node) bool {
	if n == nil {
		return false
	}
	if n.Type == html.ElementNode {
		switch strings.ToLower(n.Data) {
		case "th", "thead", "caption":
			return true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if tableLooksLikeData(c) {
			return true
		}
	}
	return false
}

func tableAlignClass(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "left":
		return "align-left"
	case "center", "middle":
		return "align-center"
	case "right":
		return "align-right"
	case "justify":
		return "align-justify"
	}
	return ""
}

func tableValignClass(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "top":
		return "valign-top"
	case "middle":
		return "valign-middle"
	case "bottom":
		return "valign-bottom"
	}
	return ""
}

func appendClass(attrs []html.Attribute, extra string) []html.Attribute {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return attrs
	}
	for i, a := range attrs {
		if !strings.EqualFold(a.Key, "class") {
			continue
		}
		cur := strings.TrimSpace(a.Val)
		if cur == "" {
			attrs[i].Val = extra
			return attrs
		}
		for _, part := range strings.Fields(cur) {
			if part == extra {
				return attrs
			}
		}
		attrs[i].Val = cur + " " + extra
		return attrs
	}
	return append(attrs, html.Attribute{Key: "class", Val: extra})
}

func rewriteAnchor(href, chapterHref string) []html.Attribute {
	if href == "" || isDangerousURL(href) {
		return nil
	}
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		return []html.Attribute{
			{Key: "href", Val: href},
			{Key: "target", Val: "_blank"},
			{Key: "rel", Val: "noopener noreferrer"},
		}
	}
	file, frag := splitFragment(href)
	attrs := []html.Attribute{{Key: "href", Val: "#"}}
	if file == "" {
		attrs = append(attrs, html.Attribute{Key: "data-fragment", Val: frag})
		return attrs
	}
	attrs = append(attrs,
		html.Attribute{Key: "data-href", Val: resolvePath(chapterHref, file)},
		html.Attribute{Key: "data-fragment", Val: frag},
	)
	return attrs
}

func rewriteResource(src, chapterHref string) (string, bool) {
	if src == "" || isDangerousURL(src) {
		return "", false
	}
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return src, true
	}
	file, _ := splitFragment(src)
	resolved := resolvePath(chapterHref, file)
	return "/res?p=" + url.QueryEscape(resolved), true
}

func isDangerousURL(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "javascript:") || strings.HasPrefix(v, "vbscript:")
}
