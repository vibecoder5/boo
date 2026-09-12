package dict

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

type Index struct {
	Name     string
	FromLang string
	ToLang   string
	words    map[string][]string
}

type Hit struct {
	Word string `json:"word"`
	Text string `json:"text"`
}

func (idx *Index) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.words)
}

func (idx *Index) Lookup(query string) []Hit {
	if idx == nil || len(idx.words) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []Hit
	for _, key := range lookupKeys(query) {
		for _, text := range idx.words[key] {
			if text == "" || seen[key+"\n"+text] {
				continue
			}
			seen[key+"\n"+text] = true
			out = append(out, Hit{Word: key, Text: text})
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func Parse(name string, data []byte) (*Index, error) {
	text := decodeText(data)
	text = strings.TrimPrefix(text, "\ufeff")
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("пустой файл")
	}
	idx := &Index{words: map[string][]string{}}
	kind := detectFormat(name, text)
	switch kind {
	case "dsl":
		parseDSL(idx, text)
	case "xdxf":
		parseXDXF(idx, text)
	case "json":
		if err := parseJSON(idx, text); err != nil {
			return nil, err
		}
	default:
		parseLines(idx, text)
	}
	if idx.Len() == 0 {
		return nil, fmt.Errorf("в файле нет словарных статей")
	}
	if idx.Name == "" {
		idx.Name = defaultName(name, idx.FromLang, idx.ToLang)
	}
	if idx.FromLang == "" || idx.ToLang == "" {
		from, to := langsFromName(name + " " + idx.Name)
		if idx.FromLang == "" {
			idx.FromLang = from
		}
		if idx.ToLang == "" {
			idx.ToLang = to
		}
	}
	return idx, nil
}

func detectFormat(name, text string) string {
	ext := strings.ToLower(filepath.Ext(name))
	head := strings.TrimSpace(text)
	if len(head) > 800 {
		head = head[:800]
	}
	low := strings.ToLower(head)
	switch ext {
	case ".dsl":
		return "dsl"
	case ".xdxf":
		return "xdxf"
	case ".json":
		return "json"
	}
	if strings.Contains(low, "#name") || strings.Contains(low, "#index_language") || strings.Contains(low, "#contents_language") {
		return "dsl"
	}
	if strings.Contains(low, "<xdxf") || strings.Contains(low, "<ar>") {
		return "xdxf"
	}
	if strings.HasPrefix(head, "{") || strings.HasPrefix(head, "[") {
		return "json"
	}
	return "lines"
}

func defaultName(file, from, to string) string {
	base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
	if base != "" && !strings.EqualFold(base, "dictionary") && base != "dict" {
		return base
	}
	if from != "" && to != "" {
		return strings.ToUpper(from) + " → " + strings.ToUpper(to)
	}
	return "Словарь"
}

func langsFromName(s string) (from, to string) {
	low := strings.ToLower(s)
	low = strings.ReplaceAll(low, "_", "-")
	low = strings.ReplaceAll(low, " ", "-")
	pairs := [][2]string{
		{"en-ru", "en"}, {"ru-en", "ru"},
		{"en-de", "en"}, {"de-en", "de"},
		{"en-fr", "en"}, {"fr-en", "fr"},
		{"en-es", "en"}, {"es-en", "es"},
		{"de-ru", "de"}, {"ru-de", "ru"},
		{"fr-ru", "fr"}, {"ru-fr", "ru"},
		{"uk-ru", "uk"}, {"ru-uk", "ru"},
	}
	rest := map[string]string{
		"en-ru": "ru", "ru-en": "en",
		"en-de": "de", "de-en": "en",
		"en-fr": "fr", "fr-en": "en",
		"en-es": "es", "es-en": "en",
		"de-ru": "ru", "ru-de": "de",
		"fr-ru": "ru", "ru-fr": "fr",
		"uk-ru": "ru", "ru-uk": "uk",
	}
	for _, p := range pairs {
		if strings.Contains(low, p[0]) {
			return p[1], rest[p[0]]
		}
	}
	switch {
	case strings.Contains(low, "англ") && strings.Contains(low, "рус"):
		if strings.Index(low, "рус") < strings.Index(low, "англ") {
			return "ru", "en"
		}
		return "en", "ru"
	case strings.Contains(low, "english") && strings.Contains(low, "russian"):
		if strings.Index(low, "russian") < strings.Index(low, "english") {
			return "ru", "en"
		}
		return "en", "ru"
	}
	return "", ""
}

func decodeText(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16(data[2:], false)
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return decodeUTF16(data[2:], true)
	}
	if looksUTF16LE(data) {
		return decodeUTF16(data, false)
	}
	if !utf8.Valid(data) && len(data)%2 == 0 {
		return decodeUTF16(data, false)
	}
	return string(data)
}

func looksUTF16LE(data []byte) bool {
	if len(data) < 8 || len(data)%2 != 0 {
		return false
	}
	n := 40
	if n*2 > len(data) {
		n = len(data) / 2
	}
	zeros := 0
	for i := 0; i < n; i++ {
		if data[i*2+1] == 0 {
			zeros++
		}
	}
	return zeros >= n/2
}

func decodeUTF16(data []byte, big bool) string {
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	u := make([]uint16, len(data)/2)
	for i := range u {
		if big {
			u[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
		} else {
			u[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
		}
	}
	return string(utf16.Decode(u))
}

func parseDSL(idx *Index, text string) {
	lines := splitLines(text)
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			parseDSLHeader(idx, line)
			i++
			continue
		}
		break
	}
	var head string
	var body []string
	flush := func() {
		head = strings.TrimSpace(head)
		if head == "" || len(body) == 0 {
			head, body = "", nil
			return
		}
		text := cleanArticle(strings.Join(body, "\n"))
		if text == "" {
			head, body = "", nil
			return
		}
		for _, key := range headKeys(head) {
			idx.add(key, text)
		}
		head, body = "", nil
	}
	for ; i < len(lines); i++ {
		raw := strings.TrimRight(lines[i], "\r")
		if raw == "" {
			flush()
			continue
		}
		if strings.HasPrefix(raw, "#") && head == "" {
			parseDSLHeader(idx, strings.TrimSpace(raw))
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			if head != "" {
				body = append(body, strings.TrimSpace(raw))
			}
			continue
		}
		flush()
		head = raw
	}
	flush()
}

func parseDSLHeader(idx *Index, line string) {
	if !strings.HasPrefix(line, "#") {
		return
	}
	key, val, ok := strings.Cut(strings.TrimSpace(line[1:]), " ")
	if !ok {
		return
	}
	val = strings.Trim(val, `"'`)
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "name":
		if idx.Name == "" {
			idx.Name = strings.TrimSpace(val)
		}
	case "index_language":
		idx.FromLang = normalizeLang(val)
	case "contents_language":
		idx.ToLang = normalizeLang(val)
	}
}

var (
	xdxfArticle = regexp.MustCompile(`(?is)<ar\b[^>]*>\s*<k\b[^>]*>(.*?)</k>(.*?)</ar>`)
	xmlTag      = regexp.MustCompile(`(?s)<[^>]+>`)
	dslTag      = regexp.MustCompile(`(?s)\[[^\]]*\]`)
	dslOptional = regexp.MustCompile(`\{([^}]*)\}`)
	dslComment  = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
)

func parseXDXF(idx *Index, text string) {
	for _, m := range xdxfArticle.FindAllStringSubmatch(text, -1) {
		head := cleanArticle(stripXML(m[1]))
		body := cleanArticle(stripXML(m[2]))
		if head == "" || body == "" {
			continue
		}
		for _, key := range headKeys(head) {
			idx.add(key, body)
		}
	}
}

func parseJSON(idx *Index, text string) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(strings.TrimSpace(text))))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("неверный JSON: %w", err)
	}
	switch v := raw.(type) {
	case map[string]any:
		for k, val := range v {
			if text := jsonText(val); text != "" {
				idx.add(k, text)
			}
		}
	case []any:
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			head := firstString(obj, "word", "w", "k", "from", "head", "headword", "src")
			body := firstString(obj, "text", "t", "v", "to", "tr", "translation", "def", "definition")
			if head != "" && body != "" {
				idx.add(head, body)
			}
		}
	default:
		return fmt.Errorf("неверный JSON-словарь")
	}
	return nil
}

func jsonText(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		var parts []string
		for _, item := range t {
			if s := jsonText(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		return firstString(t, "text", "t", "tr", "translation", "def", "definition")
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstString(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := jsonText(obj[k]); s != "" {
			return s
		}
	}
	return ""
}

func parseLines(idx *Index, text string) {
	for _, raw := range splitLines(text) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			if strings.HasPrefix(line, "#") {
				parseDSLHeader(idx, line)
			}
			continue
		}
		head, body, ok := splitEntry(line)
		if !ok {
			continue
		}
		head, body = strings.TrimSpace(head), cleanArticle(body)
		if head == "" || body == "" {
			continue
		}
		for _, key := range headKeys(head) {
			idx.add(key, body)
		}
	}
}

func splitEntry(line string) (string, string, bool) {
	for _, sep := range []string{"\t", " | ", " — ", " – ", " = "} {
		if i := strings.Index(line, sep); i > 0 {
			return line[:i], line[i+len(sep):], true
		}
	}
	if i := strings.Index(line, "|"); i > 0 {
		return line[:i], line[i+1:], true
	}
	return "", "", false
}

func (idx *Index) add(head, text string) {
	key := normalizeWord(head)
	if key == "" || text == "" {
		return
	}
	for _, old := range idx.words[key] {
		if old == text {
			return
		}
	}
	idx.words[key] = append(idx.words[key], text)
}

func headKeys(head string) []string {
	head = strings.TrimSpace(head)
	if head == "" {
		return nil
	}
	full := dslOptional.ReplaceAllString(head, "$1")
	bare := dslOptional.ReplaceAllString(head, "")
	out := []string{full}
	if bare != full {
		out = append(out, bare)
	}
	return out
}

func lookupKeys(q string) []string {
	q = normalizeWord(q)
	if q == "" {
		return nil
	}
	seen := map[string]bool{q: true}
	out := []string{q}
	add := func(s string) {
		s = normalizeWord(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	if strings.Contains(q, " ") {
		add(strings.ReplaceAll(q, " ", "-"))
		add(strings.ReplaceAll(q, " ", ""))
	}
	for _, alt := range stemVariants(q) {
		add(alt)
	}
	return out
}

func normalizeWord(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "ʼ", "'")
	s = strings.Trim(s, " \t.,;:!?¡¿\"'«»()[]{}<>…“”„—–-")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func stemVariants(q string) []string {
	var out []string
	add := func(s string) {
		if s != "" && s != q && utf8.RuneCountInString(s) >= 3 {
			out = append(out, s)
		}
	}
	switch {
	case strings.HasSuffix(q, "ies") && len(q) > 5:
		add(strings.TrimSuffix(q, "ies") + "y")
	case strings.HasSuffix(q, "ied") && len(q) > 5:
		add(strings.TrimSuffix(q, "ied") + "y")
	}
	for _, suf := range []string{"'s", "s", "es", "ly", "er", "est"} {
		if strings.HasSuffix(q, suf) && utf8.RuneCountInString(q) > len(suf)+2 {
			add(strings.TrimSuffix(q, suf))
		}
	}
	for _, suf := range []string{"ing", "ed"} {
		if !strings.HasSuffix(q, suf) || utf8.RuneCountInString(q) <= len(suf)+2 {
			continue
		}
		stem := strings.TrimSuffix(q, suf)
		add(stem)
		add(stem + "e")
		if r := []rune(stem); len(r) >= 2 && r[len(r)-1] == r[len(r)-2] && isConsonant(r[len(r)-1]) {
			add(string(r[:len(r)-1]))
		}
	}
	if !hasCyrillic(q) {
		return out
	}
	var stems []string
	for _, suf := range []string{
		"ами", "ями", "ого", "его", "ому", "ему", "ыми", "ими",
		"ой", "ей", "ою", "ею", "ов", "ев", "ам", "ям", "ах", "ях",
		"ую", "юю", "ая", "яя", "ые", "ие", "ых", "их", "ть",
		"ла", "ло", "ли", "а", "я", "ы", "и", "у", "ю", "о", "е", "й",
	} {
		if strings.HasSuffix(q, suf) && utf8.RuneCountInString(q) > utf8.RuneCountInString(suf)+2 {
			stem := strings.TrimSuffix(q, suf)
			add(stem)
			stems = append(stems, stem)
		}
	}
	for _, stem := range stems {
		for _, end := range []string{"а", "я", "ь", "о", "е", "й", "ть", "ие"} {
			add(stem + end)
		}
	}
	return out
}

func isConsonant(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return false
	}
	return r >= 'a' && r <= 'z'
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Cyrillic) {
			return true
		}
	}
	return false
}

func normalizeLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, `"'`)
	switch {
	case s == "en" || strings.HasPrefix(s, "eng") || s == "английский":
		return "en"
	case s == "ru" || strings.HasPrefix(s, "rus") || s == "русский":
		return "ru"
	case s == "de" || strings.HasPrefix(s, "ger") || strings.HasPrefix(s, "deu") || s == "немецкий":
		return "de"
	case s == "fr" || strings.HasPrefix(s, "fre") || strings.HasPrefix(s, "fra") || s == "французский":
		return "fr"
	case s == "es" || strings.HasPrefix(s, "spa") || s == "испанский":
		return "es"
	case s == "it" || strings.HasPrefix(s, "ita") || s == "итальянский":
		return "it"
	case s == "uk" || strings.HasPrefix(s, "ukr") || s == "украинский":
		return "uk"
	case s == "pl" || strings.HasPrefix(s, "pol") || s == "польский":
		return "pl"
	case s == "zh" || strings.HasPrefix(s, "chi") || s == "китайский":
		return "zh"
	case s == "ja" || strings.HasPrefix(s, "jpn") || s == "японский":
		return "ja"
	}
	if len(s) >= 2 {
		return s[:2]
	}
	return s
}

func cleanArticle(s string) string {
	s = dslComment.ReplaceAllString(s, "")
	s = dslTag.ReplaceAllString(s, "")
	s = stripXML(s)
	s = strings.ReplaceAll(s, "\\ ", " ")
	s = strings.ReplaceAll(s, "\\[", "[")
	s = strings.ReplaceAll(s, "\\]", "]")
	s = strings.ReplaceAll(s, "{{", "")
	s = strings.ReplaceAll(s, "}}", "")
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	return strings.TrimSpace(s)
}

func stripXML(s string) string {
	s = xmlTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	return s
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}
