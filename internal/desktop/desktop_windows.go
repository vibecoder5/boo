//go:build windows

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

func Supported() bool { return true }

func Run(appURL, title string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	dataPath := filepath.Join(dir, "boo", "webview")
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return err
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle(title),
			Width:  1200,
			Height: 800,
			Center: true,
		},
	})
	if w == nil {
		return fmt.Errorf("не удалось открыть окно. Установите Microsoft Edge WebView2 Runtime")
	}
	defer w.Destroy()
	noteWindow(w.Window())
	w.SetSize(880, 600, webview2.HintMin)
	showMaximized(w.Window())
	w.Navigate(appURL)
	w.Run()
	return nil
}

const swShowMaximized = 3

func showMaximized(hwnd unsafe.Pointer) {
	if hwnd == nil {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	_, _, _ = user32.NewProc("ShowWindow").Call(uintptr(hwnd), swShowMaximized)
}
