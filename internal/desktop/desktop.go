package desktop

import "strings"

func windowTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "boo"
	}
	return title + " — boo"
}
