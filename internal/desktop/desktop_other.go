//go:build !windows

package desktop

import "fmt"

func Supported() bool { return false }

func Run(appURL, title string) error {
	return fmt.Errorf("окно приложения на этой системе пока не поддерживается, откройте %s в браузере", appURL)
}
