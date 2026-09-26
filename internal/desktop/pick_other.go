//go:build !windows

package desktop

import "errors"

func CanPickFolder() bool { return false }

func PickFolder(string) (string, bool, error) {
	return "", false, errors.New("выбор папки доступен в окне Windows")
}
