package open

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"boo/internal/epub"
	"boo/internal/fb2"
	"boo/internal/txt"
)

func Open(path string) (*epub.Book, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".epub":
		return epub.Open(path)
	case ".fb2":
		return fb2.Open(path)
	case ".txt", ".text", ".md", ".markdown":
		return txt.Open(path)
	case ".zip":
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
	default:
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
}

func OpenBytes(name string, data []byte) (*epub.Book, error) {
	if looksZip(data) {
		if isEPUB(data) {
			book, err := epub.OpenBytes(name, data)
			if err == nil && book.Format == "" {
				book.Format = "epub"
			}
			return book, err
		}
		return fb2.OpenZipBytes(name, data)
	}
	if fb2.LooksLike(data) {
		return fb2.OpenBytes(name, data)
	}
	if txt.LooksLike(name, data) {
		return txt.OpenBytes(name, data)
	}
	return nil, fmt.Errorf("неизвестный формат: нужен EPUB, FB2 или TXT")
}

func looksZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K'
}

func isEPUB(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, zf := range zr.File {
		name := strings.ToLower(strings.ReplaceAll(zf.Name, "\\", "/"))
		if name == "meta-inf/container.xml" || strings.HasSuffix(name, "meta-inf/container.xml") {
			return true
		}
	}
	return false
}
