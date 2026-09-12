package fb2

import (
	"encoding/xml"
	"strings"
)

type xnode struct {
	kind  byte
	local string
	attr  []xml.Attr
	text  string
	kids  []*xnode
}

func readElem(dec *xml.Decoder, start xml.StartElement) (*xnode, error) {
	n := &xnode{kind: 'e', local: strings.ToLower(start.Name.Local), attr: start.Attr}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kid, err := readElem(dec, t)
			if err != nil {
				return nil, err
			}
			n.kids = append(n.kids, kid)
		case xml.CharData:
			n.kids = append(n.kids, &xnode{kind: 't', text: string(t)})
		case xml.EndElement:
			return n, nil
		}
	}
}

func (n *xnode) attrVal(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func (n *xnode) plain() string {
	if n == nil {
		return ""
	}
	if n.kind == 't' {
		return n.text
	}
	var b strings.Builder
	for _, kid := range n.kids {
		b.WriteString(kid.plain())
	}
	return b.String()
}

func find(n *xnode, local string) *xnode {
	if n == nil {
		return nil
	}
	if n.kind == 'e' && n.local == local {
		return n
	}
	for _, kid := range n.kids {
		if found := find(kid, local); found != nil {
			return found
		}
	}
	return nil
}

func findAll(n *xnode, local string) []*xnode {
	var out []*xnode
	if n == nil {
		return out
	}
	if n.kind == 'e' && n.local == local {
		out = append(out, n)
	}
	for _, kid := range n.kids {
		out = append(out, findAll(kid, local)...)
	}
	return out
}
