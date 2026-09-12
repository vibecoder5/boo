package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

const SampleIdentifier = "urn:uuid:boo-demo"

func Sample() ([]byte, error) {
	cover, err := sampleCover()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := storeFile(zw, "mimetype", []byte("application/epub+zip")); err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"META-INF/container.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`),
		"OEBPS/content.opf": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="bookid" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">` + SampleIdentifier + `</dc:identifier>
    <dc:title>Записки на полях</dc:title>
    <dc:creator>boo</dc:creator>
    <dc:language>ru</dc:language>
    <meta name="cover" content="cover"/>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch3" href="ch3.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
    <itemref idref="ch3"/>
  </spine>
</package>
`),
		"OEBPS/toc.ncx": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="n1" playOrder="1">
      <navLabel><text>Начало</text></navLabel>
      <content src="ch1.xhtml"/>
    </navPoint>
    <navPoint id="n2" playOrder="2">
      <navLabel><text>Как этим пользоваться</text></navLabel>
      <content src="ch2.xhtml"/>
      <navPoint id="n2a" playOrder="3">
        <navLabel><text>Клавиши</text></navLabel>
        <content src="ch2.xhtml#keys"/>
      </navPoint>
      <navPoint id="n2b" playOrder="4">
        <navLabel><text>Картинки</text></navLabel>
        <content src="ch2.xhtml#pics"/>
      </navPoint>
    </navPoint>
    <navPoint id="n3" playOrder="5">
      <navLabel><text>Дальше</text></navLabel>
      <content src="ch3.xhtml"/>
    </navPoint>
  </navMap>
</ncx>
`),
		"OEBPS/nav.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<body>
<nav epub:type="toc">
  <ol>
    <li><a href="ch1.xhtml">Начало</a></li>
    <li>
      <a href="ch2.xhtml">Как этим пользоваться</a>
      <ol>
        <li><a href="ch2.xhtml#keys">Клавиши</a></li>
        <li><a href="ch2.xhtml#pics">Картинки</a></li>
      </ol>
    </li>
    <li><a href="ch3.xhtml">Дальше</a></li>
  </ol>
</nav>
</body>
</html>
`),
		"OEBPS/ch1.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Начало</title></head>
<body>
<h1>Начало</h1>
<p>Это встроенная демо-книга читалки <em>boo</em>. Она нужна только для того, чтобы сразу увидеть, как выглядит текст, оглавление и смена темы.</p>
<p>Откройте свой файл EPUB кнопкой «+» на полке, через контекстное меню или перетащите его в окно. Прогресс, размер шрифта и тема сохраняются локально.</p>
<p>Чтение лучше всего получается, когда странице не нужно ничего доказывать. Поэтому здесь мало украшений и много воздуха.</p>
</body>
</html>
`),
		"OEBPS/ch2.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Как этим пользоваться</title></head>
<body>
<h1>Как этим пользоваться</h1>
<script>alert("xss")</script>
<p>Слева — оглавление. Его можно свернуть целиком или закрыть отдельные главы со вложенными пунктами. Стрелки на клавиатуре листают главы, клавиши <code>[</code> и <code>]</code> меняют размер шрифта, <code>t</code> переключает тему. Таблицы из EPUB читалка рисует отдельно от обычного текста.</p>
<h2 id="keys">Клавиши</h2>
<table>
  <caption>Клавиши и жесты</caption>
  <thead>
    <tr>
      <th scope="col">Клавиша</th>
      <th scope="col">Действие</th>
      <th scope="col" align="right">Где</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <th scope="row"><code>/</code></th>
      <td>Поиск по книге</td>
      <td align="right">меню</td>
    </tr>
    <tr>
      <th scope="row"><code>b</code></th>
      <td>Поставить закладку</td>
      <td align="right">чтение</td>
    </tr>
    <tr>
      <th scope="row"><code>t</code></th>
      <td>Сменить тему</td>
      <td align="right">везде</td>
    </tr>
    <tr>
      <th scope="row"><code>[</code> <code>]</code></th>
      <td>Размер шрифта книги</td>
      <td align="right">чтение</td>
    </tr>
    <tr>
      <th scope="row">ПКМ</th>
      <td>Цветное выделение</td>
      <td align="right">текст</td>
    </tr>
  </tbody>
</table>
<figure id="pics">
  <img src="cover.png" alt="Обложка демо-книги"/>
  <figcaption>Картинки из EPUB тоже показываются.</figcaption>
</figure>
<p>Внутренние ссылки работают: <a href="ch3.xhtml">перейти к следующей главе</a>.</p>
</body>
</html>
`),
		"OEBPS/ch3.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Дальше</title></head>
<body>
<h1>Дальше</h1>
<p>Когда появится настоящая книга, boo запомнит главу и положение на странице. Можно закрыть окно и вернуться позже.</p>
<p>Форматы вроде FB2, пагинация «как на бумаге» и библиотека на диске — уже следующий шаг. Сейчас это намеренно маленький MVP: открыть EPUB и спокойно читать.</p>
<blockquote><p>Лучшая читалка та, которую не замечаешь.</p></blockquote>
</body>
</html>
`),
		"OEBPS/cover.png": cover,
	}
	for name, data := range files {
		if err := deflateFile(zw, name, data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func storeFile(zw *zip.Writer, name string, data []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Store}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func deflateFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func sampleCover() ([]byte, error) {
	const w, h = 320, 480
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	paper := color.RGBA{R: 46, G: 38, B: 31, A: 255}
	band := color.RGBA{R: 196, G: 149, B: 92, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < 18 || x > w-19 {
				img.Set(x, y, band)
			} else {
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
