package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type zipReader interface {
	file(name string) *zipFile
}

type zipFile struct {
	name        string
	contentType string
	open        func() (io.ReadCloser, error)
}

func (f *zipFile) read() ([]byte, error) {
	rc, err := f.open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

type zipMap map[string]*zipFile

func (m zipMap) file(name string) *zipFile {
	return m[normZipPath(name)]
}

func Open(path string) (*Book, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("not an EPUB (zip): %w", err)
	}
	book, err := fromZip(zr, path, info.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	book.closer = f
	return book, nil
}

func OpenBytes(name string, data []byte) (*Book, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not an EPUB (zip): %w", err)
	}
	return fromZip(zr, name, int64(len(data)))
}

func fromZip(zr *zip.Reader, origin string, size int64) (*Book, error) {
	files := make(zipMap, len(zr.File))
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		name := normZipPath(zf.Name)
		files[name] = &zipFile{
			name: name,
			open: zf.Open,
		}
	}

	container, err := readFile(files, "META-INF/container.xml")
	if err != nil {
		if alt := findFileCI(files, "META-INF/container.xml"); alt != "" {
			container, err = readFile(files, alt)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("missing container.xml: %w", err)
	}

	opfPath, err := parseContainer(container)
	if err != nil {
		return nil, err
	}
	opfPath = normZipPath(opfPath)

	opfData, err := readFile(files, opfPath)
	if err != nil {
		return nil, fmt.Errorf("missing OPF %s: %w", opfPath, err)
	}

	pkg, err := parseOPF(opfData)
	if err != nil {
		return nil, err
	}

	itemByID := make(map[string]opfItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		item.Href = resolvePath(opfPath, item.Href)
		itemByID[item.ID] = item
		if f := files[item.Href]; f != nil && item.MediaType != "" {
			f.contentType = item.MediaType
		}
	}

	book := &Book{
		Path:       origin,
		Title:      firstNonEmpty(pkg.Title, fallbackTitle(origin)),
		Author:     pkg.Author,
		Language:   pkg.Language,
		Identifier: pkg.Identifier,
		Format:     "epub",
		zr:         files,
		files:      files,
	}

	for _, ref := range pkg.Spine {
		item, ok := itemByID[ref]
		if !ok {
			continue
		}
		if !isDocument(item.MediaType, item.Href) {
			continue
		}
		book.Chapters = append(book.Chapters, Chapter{
			ID:    item.ID,
			Href:  item.Href,
			Title: "",
		})
	}
	if len(book.Chapters) == 0 {
		return nil, fmt.Errorf("EPUB has no readable chapters")
	}

	book.CoverHref = findCover(pkg, itemByID)

	var toc []TOCItem
	if pkg.NavHref != "" {
		navPath := resolvePath(opfPath, pkg.NavHref)
		if data, err := readFile(files, navPath); err == nil {
			toc = parseNav(data, navPath)
		}
	}
	if len(toc) == 0 && pkg.NCXID != "" {
		if item, ok := itemByID[pkg.NCXID]; ok {
			if data, err := readFile(files, item.Href); err == nil {
				toc = parseNCX(data, item.Href)
			}
		}
	}
	if len(toc) == 0 {
		for _, item := range pkg.Manifest {
			if strings.EqualFold(path.Ext(item.Href), ".ncx") {
				if data, err := readFile(files, item.Href); err == nil {
					toc = parseNCX(data, item.Href)
					break
				}
			}
		}
	}

	book.TOC = bindTOC(toc, book)
	fillChapterTitles(book)
	book.Key = bookKey(book.Identifier, book.Title, origin, size)
	return book, nil
}

func isDocument(mediaType, href string) bool {
	switch strings.ToLower(mediaType) {
	case "application/xhtml+xml", "text/html", "application/x-dtbook+xml":
		return true
	}
	switch strings.ToLower(path.Ext(href)) {
	case ".xhtml", ".html", ".htm", ".xml":
		return !strings.EqualFold(path.Base(href), "container.xml")
	}
	return false
}

func findCover(pkg *opfPackage, items map[string]opfItem) string {
	if pkg.CoverID != "" {
		if item, ok := items[pkg.CoverID]; ok {
			return item.Href
		}
	}
	for _, item := range items {
		props := strings.Fields(strings.ToLower(item.Properties))
		for _, p := range props {
			if p == "cover-image" {
				return item.Href
			}
		}
	}
	return ""
}

func fillChapterTitles(book *Book) {
	titles := map[string]string{}
	var walk func([]TOCItem)
	walk = func(items []TOCItem) {
		for _, it := range items {
			if it.Href != "" && it.Title != "" {
				if _, ok := titles[it.Href]; !ok {
					titles[it.Href] = it.Title
				}
			}
			walk(it.Children)
		}
	}
	walk(book.TOC)
	for i := range book.Chapters {
		if title := titles[book.Chapters[i].Href]; title != "" {
			book.Chapters[i].Title = title
			continue
		}
		book.Chapters[i].Title = prettyFileTitle(book.Chapters[i].Href, i)
	}
}

func bindTOC(items []TOCItem, book *Book) []TOCItem {
	out := make([]TOCItem, len(items))
	for i, it := range items {
		it.ChapterIndex = book.ChapterIndexByHref(it.Href)
		it.Children = bindTOC(it.Children, book)
		out[i] = it
	}
	return out
}

func prettyFileTitle(href string, index int) string {
	base := path.Base(href)
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.TrimSpace(base)
	if base == "" {
		return fmt.Sprintf("Глава %d", index+1)
	}
	return base
}

func fallbackTitle(origin string) string {
	base := path.Base(strings.ReplaceAll(origin, "\\", "/"))
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "." {
		return "Без названия"
	}
	return base
}

func bookKey(id, title, origin string, size int64) string {
	if strings.TrimSpace(id) != "" {
		return "id:" + strings.TrimSpace(id)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", origin, title, size)))
	return "file:" + hex.EncodeToString(sum[:16])
}

func readFile(files zipMap, name string) ([]byte, error) {
	f := files[normZipPath(name)]
	if f == nil {
		return nil, errNotFound(name)
	}
	return f.read()
}

func findFileCI(files zipMap, name string) string {
	want := strings.ToLower(normZipPath(name))
	for n := range files {
		if strings.ToLower(n) == want {
			return n
		}
	}
	return ""
}

func normZipPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func splitFragment(href string) (string, string) {
	href = strings.TrimSpace(href)
	if i := strings.IndexByte(href, '#'); i >= 0 {
		return href[:i], href[i+1:]
	}
	return href, ""
}

func resolvePath(baseFile, href string) string {
	href, _ = splitFragment(href)
	if i := strings.IndexByte(href, '?'); i >= 0 {
		href = href[:i]
	}
	href = strings.TrimSpace(href)
	if href == "" {
		return normZipPath(baseFile)
	}
	if strings.Contains(href, "://") {
		return href
	}
	dir := path.Dir(baseFile)
	if dir == "." {
		dir = ""
	}
	return normZipPath(path.Join(dir, href))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type missingError struct{ name string }

func (e missingError) Error() string { return fmt.Sprintf("%s not found", e.name) }

func errNotFound(name string) error { return missingError{name: name} }
