//go:build !windows

package install

import "fmt"

func WebView2Installed() bool { return false }

func Install(Options) (Result, error) {
	return Result{}, fmt.Errorf("установщик только для Windows")
}

func Uninstall(Profile) error {
	return fmt.Errorf("установщик только для Windows")
}

func Installed(Profile) (string, bool) { return "", false }
