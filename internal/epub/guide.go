package epub

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
)

const GuideIdentifier = "urn:uuid:boo-guide"

func Guide(markdown []byte) (*Book, error) {
	title, chapters, err := parseMarkdown(string(markdown))
	if err != nil {
		return nil, err
	}
	cover, err := guideCover()
	if err != nil {
		return nil, err
	}

	book := NewMemory("guide.md", "guide")
	book.Title = title
	book.Author = "boo"
	book.Language = "ru"
	book.Identifier = GuideIdentifier
	book.CoverHref = "cover.png"
	book.Put("cover.png", "image/png", cover)

	var toc []TOCItem
	for i, ch := range chapters {
		href := fmt.Sprintf("c%04d.xhtml", i+1)
		body := "<h1>" + html.EscapeString(ch.Title) + "</h1>\n" + ch.html()
		book.AddChapter(fmt.Sprintf("c%d", i+1), href, ch.Title, wrapGuide(ch.Title, body))
		item := TOCItem{Title: ch.Title, Href: href}
		for _, sub := range ch.Subs {
			item.Children = append(item.Children, TOCItem{
				Title:    sub.Title,
				Href:     href,
				Fragment: sub.ID,
			})
		}
		toc = append(toc, item)
	}
	book.SetTOC(toc)
	book.Finish(int64(len(markdown)))
	return book, nil
}

func wrapGuide(title, body string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>` +
		html.EscapeString(title) + `</title></head><body>` + body + `</body></html>`)
}

func guideCover() ([]byte, error) {
	const w, h = 320, 480
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	paper := color.RGBA{R: 52, G: 44, B: 36, A: 255}
	band := color.RGBA{R: 168, G: 112, B: 72, A: 255}
	rule := color.RGBA{R: 196, G: 149, B: 92, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < 16 || x > w-17:
				img.Set(x, y, band)
			case y > 70 && y < 74:
				img.Set(x, y, rule)
			default:
				img.Set(x, y, paper)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("cover: %w", err)
	}
	return buf.Bytes(), nil
}
