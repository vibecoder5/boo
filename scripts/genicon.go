//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	sizes := []int{16, 32, 48, 256}
	var blobs [][]byte
	for _, s := range sizes {
		img := book(s)
		if s >= 256 {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				panic(err)
			}
			blobs = append(blobs, buf.Bytes())
			continue
		}
		blobs = append(blobs, dib32(img))
	}
	out := filepath.Join("install", "icon.ico")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(out, encodeICO(sizes, blobs), 0o644); err != nil {
		panic(err)
	}
	var preview bytes.Buffer
	if err := png.Encode(&preview, book(128)); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(os.TempDir(), "boo-icon-preview.png"), preview.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

func book(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0x14, 0x11, 0x0e, 255}
	paper := color.RGBA{0xec, 0xe4, 0xd8, 255}
	accent := color.RGBA{0xc4, 0x95, 0x5c, 255}
	ink := color.RGBA{0x3b, 0x2a, 0x1a, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{0, 0, 0, 0}}, image.Point{}, draw.Src)

	m := size / 16
	if m < 1 {
		m = 1
	}
	fillRound(img, m, m, size-m, size-m, m*2, bg)
	left := m * 3
	right := size - m*3
	top := m * 4
	bot := size - m*3
	mid := size / 2
	fillRect(img, left, top, mid-m/2, bot, paper)
	fillRect(img, mid+m/2, top, right, bot, paper)
	fillRect(img, mid-m/2, top, mid+m/2, bot, accent)
	for i := 0; i < 3; i++ {
		y := top + m*2 + i*m*2
		if y >= bot-m {
			break
		}
		fillRect(img, left+m, y, mid-m, y+m/2, ink)
		fillRect(img, mid+m, y, right-m, y+m/2, ink)
	}
	return img
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if img.Bounds().Overlaps(image.Rect(x, y, x+1, y+1)) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillRound(img *image.RGBA, x0, y0, x1, y1, r int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if insideRound(x, y, x0, y0, x1, y1, r) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func insideRound(x, y, x0, y0, x1, y1, r int) bool {
	cx, cy := x, y
	if x < x0+r && y < y0+r {
		cx, cy = x-(x0+r), y-(y0+r)
		return cx*cx+cy*cy <= r*r
	}
	if x >= x1-r && y < y0+r {
		cx, cy = x-(x1-r-1), y-(y0+r)
		return cx*cx+cy*cy <= r*r
	}
	if x < x0+r && y >= y1-r {
		cx, cy = x-(x0+r), y-(y1-r-1)
		return cx*cx+cy*cy <= r*r
	}
	if x >= x1-r && y >= y1-r {
		cx, cy = x-(x1-r-1), y-(y1-r-1)
		return cx*cx+cy*cy <= r*r
	}
	return true
}

func dib32(img *image.RGBA) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	xor := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		srcY := h - 1 - y
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, srcY)
			i := (y*w + x) * 4
			xor[i+0] = c.B
			xor[i+1] = c.G
			xor[i+2] = c.R
			xor[i+3] = c.A
		}
	}
	andRow := ((w + 31) / 32) * 4
	and := make([]byte, andRow*h)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(w))
	_ = binary.Write(&buf, binary.LittleEndian, int32(h*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(xor)))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	buf.Write(xor)
	buf.Write(and)
	return buf.Bytes()
}

func encodeICO(sizes []int, pngs [][]byte) []byte {
	var hdr bytes.Buffer
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(0))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(1))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(len(pngs)))
	offset := 6 + 16*len(pngs)
	var entries bytes.Buffer
	for i, s := range sizes {
		w, h := s, s
		if s >= 256 {
			w, h = 0, 0
		}
		entries.WriteByte(byte(w))
		entries.WriteByte(byte(h))
		entries.WriteByte(0)
		entries.WriteByte(0)
		_ = binary.Write(&entries, binary.LittleEndian, uint16(1))
		_ = binary.Write(&entries, binary.LittleEndian, uint16(32))
		_ = binary.Write(&entries, binary.LittleEndian, uint32(len(pngs[i])))
		_ = binary.Write(&entries, binary.LittleEndian, uint32(offset))
		offset += len(pngs[i])
	}
	var out bytes.Buffer
	out.Write(hdr.Bytes())
	out.Write(entries.Bytes())
	for _, b := range pngs {
		out.Write(b)
	}
	return out.Bytes()
}
