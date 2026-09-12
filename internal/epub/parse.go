package epub

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

type opfPackage struct {
	Title      string
	Author     string
	Language   string
	Identifier string
	CoverID    string
	NCXID      string
	NavHref    string
	Manifest   []opfItem
	Spine      []string
}

type opfItem struct {
	ID         string
	Href       string
	MediaType  string
	Properties string
}

func parseContainer(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || !localEq(se.Name.Local, "rootfile") {
			continue
		}
		fullPath, media := "", ""
		for _, a := range se.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "full-path":
				fullPath = a.Value
			case "media-type":
				media = a.Value
			}
		}
		if fullPath != "" && (media == "" || strings.Contains(media, "package") || strings.Contains(media, "oebps")) {
			return fullPath, nil
		}
		if fullPath != "" {
			return fullPath, nil
		}
	}
	return "", fmtError("container.xml has no rootfile")
}

func parseOPF(data []byte) (*opfPackage, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	pkg := &opfPackage{}
	inMetadata, inManifest, inSpine := false, false, false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "metadata":
				inMetadata = true
			case "manifest":
				inManifest = true
			case "spine":
				inSpine = true
				if v := attrLocal(t, "toc"); v != "" {
					pkg.NCXID = v
				}
			case "title":
				if inMetadata && pkg.Title == "" {
					pkg.Title = readText(dec)
				}
			case "creator":
				if inMetadata && pkg.Author == "" {
					pkg.Author = readText(dec)
				}
			case "language":
				if inMetadata && pkg.Language == "" {
					pkg.Language = readText(dec)
				}
			case "identifier":
				if inMetadata && pkg.Identifier == "" {
					pkg.Identifier = readText(dec)
				}
			case "meta":
				if inMetadata {
					name := attrLocal(t, "name")
					content := attrLocal(t, "content")
					prop := attrLocal(t, "property")
					if strings.EqualFold(name, "cover") && content != "" {
						pkg.CoverID = content
					}
					if strings.EqualFold(prop, "dcterms:title") && pkg.Title == "" {
						pkg.Title = readText(dec)
					}
				}
			case "item":
				if inManifest {
					item := opfItem{
						ID:         attrLocal(t, "id"),
						Href:       attrLocal(t, "href"),
						MediaType:  attrLocal(t, "media-type"),
						Properties: attrLocal(t, "properties"),
					}
					pkg.Manifest = append(pkg.Manifest, item)
					for _, p := range strings.Fields(strings.ToLower(item.Properties)) {
						if p == "nav" {
							pkg.NavHref = item.Href
						}
					}
				}
			case "itemref":
				if inSpine {
					if idref := attrLocal(t, "idref"); idref != "" {
						pkg.Spine = append(pkg.Spine, idref)
					}
				}
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "metadata":
				inMetadata = false
			case "manifest":
				inManifest = false
			case "spine":
				inSpine = false
			}
		}
	}
	if len(pkg.Spine) == 0 {
		return nil, fmtError("OPF spine is empty")
	}
	return pkg, nil
}

func parseNCX(data []byte, ncxPath string) []TOCItem {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	type frame struct {
		item     TOCItem
		children []TOCItem
	}
	var stack []frame
	var roots []TOCItem

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return roots
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "navpoint":
				stack = append(stack, frame{})
			case "text":
				if len(stack) > 0 && stack[len(stack)-1].item.Title == "" {
					stack[len(stack)-1].item.Title = strings.TrimSpace(readText(dec))
				}
			case "content":
				if len(stack) > 0 {
					src := attrLocal(t, "src")
					href, frag := splitFragment(src)
					stack[len(stack)-1].item.Href = resolvePath(ncxPath, href)
					stack[len(stack)-1].item.Fragment = frag
				}
			}
		case xml.EndElement:
			if !localEq(t.Name.Local, "navpoint") || len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			top.item.Children = top.children
			if len(stack) == 0 {
				roots = append(roots, top.item)
			} else {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, top.item)
			}
		}
	}
	return roots
}

func parseNav(data []byte, navPath string) []TOCItem {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	nav := findTOCNav(doc)
	if nav == nil {
		return nil
	}
	ol := findChild(nav, "ol")
	if ol == nil {
		return nil
	}
	return parseNavList(ol, navPath)
}

func parseNavList(ol *html.Node, navPath string) []TOCItem {
	var items []TOCItem
	for li := ol.FirstChild; li != nil; li = li.NextSibling {
		if !isElem(li, "li") {
			continue
		}
		item := TOCItem{ChapterIndex: -1}
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if isElem(c, "a") || isElem(c, "span") {
				item.Title = strings.TrimSpace(textOf(c))
				if href := nodeAttr(c, "href"); href != "" {
					file, frag := splitFragment(href)
					item.Href = resolvePath(navPath, file)
					item.Fragment = frag
				}
			}
			if isElem(c, "ol") {
				item.Children = parseNavList(c, navPath)
			}
		}
		if item.Title != "" || item.Href != "" {
			items = append(items, item)
		}
	}
	return items
}

func findTOCNav(n *html.Node) *html.Node {
	var first *html.Node
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if isElem(n, "nav") {
			if first == nil {
				first = n
			}
			typ := strings.ToLower(nodeAttr(n, "type") + " " + nodeAttr(n, "epub:type"))
			if strings.Contains(typ, "toc") {
				return n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := walk(c); found != nil {
				return found
			}
		}
		return nil
	}
	if found := walk(n); found != nil {
		return found
	}
	return first
}

func findChild(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isElem(c, tag) {
			return c
		}
		if found := findChild(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func isElem(n *html.Node, tag string) bool {
	return n != nil && n.Type == html.ElementNode && strings.EqualFold(n.Data, tag)
}

func nodeAttr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func attrLocal(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func readText(dec *xml.Decoder) string {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return strings.TrimSpace(b.String())
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return strings.TrimSpace(b.String())
		}
	}
}

func localEq(got, want string) bool {
	return strings.EqualFold(got, want)
}

type parseError string

func (e parseError) Error() string { return string(e) }

func fmtError(msg string) error { return parseError(msg) }
