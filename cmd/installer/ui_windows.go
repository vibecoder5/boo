//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"unsafe"

	"boo/internal/install"

	"golang.org/x/sys/windows"
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsVisible     = 0x10000000
	wsChild       = 0x40000000
	wsTabStop     = 0x00010000
	wsGroup       = 0x00020000
	wsBorder      = 0x00800000
	esAutoHScroll = 0x0080
	bsAutoCheck   = 0x00000003
	bsDefPush     = 0x00000001
	colorWindow   = 5
	idcArrow      = 32512
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmSetFont     = 0x0030
	wmCommand     = 0x0111
	bmGetCheck    = 0x00F0
	bstChecked    = 1
	swShow        = 5
	smCxScreen    = 0
	smCyScreen    = 1
	mbOK          = 0x00000000
	mbYesNo       = 0x00000004
	mbIconErr     = 0x00000010
	mbIconQ       = 0x00000020
	mbIconInfo    = 0x00000040
	idYes         = 6

	idcDir    = 101
	idcBrowse = 102
	idcStart  = 103
	idcDesk   = 104
	idcAssoc  = 105
	idcOK     = 106
	idcCancel = 107
	idcWeb    = 108
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")

	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLen    = user32.NewProc("GetWindowTextLengthW")
	procSendMessage         = user32.NewProc("SendMessageW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procAdjustWindowRect    = user32.NewProc("AdjustWindowRect")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procGetDlgItem          = user32.NewProc("GetDlgItem")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procCreateFont          = gdi32.NewProc("CreateFontW")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procSHBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree       = ole32.NewProc("CoTaskMemFree")
	procCoInitialize        = ole32.NewProc("CoInitialize")
)

type uiState struct {
	opt       install.Options
	hwnd      uintptr
	cancelled bool
	err       error
	res       install.Result
}

var ui *uiState
var uiFont uintptr

func init() {
	_, _, _ = kernel32.NewProc("AttachConsole").Call(^uintptr(0))
}

func alertError(err error) {
	messageBox(0, err.Error(), "boo", mbOK|mbIconErr)
}

func runUI(opt install.Options) error {
	_, _, _ = procCoInitialize.Call(0)
	if opt.Dir == "" {
		d, err := install.DefaultDir()
		if err != nil {
			return err
		}
		opt.Dir = d
	}
	ui = &uiState{opt: opt}
	if err := showDialog(); err != nil {
		return err
	}
	if ui.cancelled {
		return nil
	}
	if ui.err != nil {
		return ui.err
	}
	return nil
}

func confirmUninstall() bool {
	return messageBox(0, "Удалить boo с компьютера?\n\nКниги, закладки и заметки в папке профиля останутся.", "boo", mbYesNo|mbIconQ) == idYes
}

func showDialog() error {
	instance, _, _ := procGetModuleHandle.Call(0)
	className, _ := windows.UTF16PtrFromString("booSetup")
	cursor, _, _ := procLoadCursor.Call(0, idcArrow)
	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(wndProc),
		hInstance:     windows.Handle(instance),
		hCursor:       windows.Handle(cursor),
		hbrBackground: colorWindow + 1,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("окно установщика: %v", err)
	}

	const cw, ch = 470, 360
	var rc rect
	rc.right, rc.bottom = cw, ch
	style := uintptr(wsOverlapped | wsCaption | wsSysMenu)
	_, _, _ = procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)
	ww := int32(rc.right - rc.left)
	wh := int32(rc.bottom - rc.top)
	sx, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sy, _, _ := procGetSystemMetrics.Call(smCyScreen)
	x := (int32(sx) - ww) / 2
	y := (int32(sy) - wh) / 2

	title, _ := windows.UTF16PtrFromString("Установка boo")
	hwnd, _, err := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		style|wsVisible, uintptr(x), uintptr(y), uintptr(ww), uintptr(wh),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("окно установщика: %v", err)
	}
	ui.hwnd = hwnd
	buildControls(hwnd, instance)
	_, _, _ = procShowWindow.Call(hwnd, swShow)
	_, _, _ = procUpdateWindow.Call(hwnd)

	var msg nativeMsg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}

func buildControls(parent, instance uintptr) {
	uiFont, _, _ = procCreateFont.Call(
		^uintptr(14-1), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(wide("Segoe UI"))),
	)
	titleFont, _, _ := procCreateFont.Call(
		^uintptr(22-1), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(wide("Segoe UI"))),
	)

	add := func(class, text string, style, x, y, w, h, id uintptr) uintptr {
		hwnd, _, _ := procCreateWindowEx.Call(
			0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(text))),
			wsChild|wsVisible|style, x, y, w, h, parent, id, instance, 0,
		)
		if uiFont != 0 {
			_, _, _ = procSendMessage.Call(hwnd, wmSetFont, uiFont, 1)
		}
		return hwnd
	}

	t := add("STATIC", "boo "+install.Version, 0, 24, 18, 420, 28, 0)
	if titleFont != 0 {
		_, _, _ = procSendMessage.Call(t, wmSetFont, titleFont, 1)
	}
	add("STATIC", "Локальная читалка. Книги остаются на этом компьютере.", 0, 24, 50, 420, 22, 0)
	add("STATIC", "Каталог", 0, 24, 86, 200, 18, 0)
	add("EDIT", ui.opt.Dir, wsBorder|wsTabStop|esAutoHScroll, 24, 108, 330, 26, idcDir)
	add("BUTTON", "Обзор…", wsTabStop, 362, 106, 84, 30, idcBrowse)
	startStyle := uintptr(bsAutoCheck | wsTabStop | wsGroup)
	deskStyle := uintptr(bsAutoCheck | wsTabStop)
	assocStyle := uintptr(bsAutoCheck | wsTabStop)
	start := add("BUTTON", "Ярлык в меню «Пуск»", startStyle, 24, 154, 400, 24, idcStart)
	desk := add("BUTTON", "Ярлык на рабочем столе", deskStyle, 24, 182, 400, 24, idcDesk)
	assoc := add("BUTTON", "Открывать EPUB и FB2 в boo", assocStyle, 24, 210, 400, 24, idcAssoc)
	if ui.opt.StartMenu {
		_, _, _ = procSendMessage.Call(start, 0x00F1, bstChecked, 0) // BM_SETCHECK
	}
	if ui.opt.Desktop {
		_, _, _ = procSendMessage.Call(desk, 0x00F1, bstChecked, 0)
	}
	if ui.opt.Associations {
		_, _, _ = procSendMessage.Call(assoc, 0x00F1, bstChecked, 0)
	}
	web := "WebView2 установлен — откроется окно читалки."
	if !install.WebView2Installed() {
		web = "WebView2 не найден: окно может не открыться. Тогда запуск с флагом -web."
	}
	add("STATIC", web, 0, 24, 248, 420, 36, idcWeb)
	add("BUTTON", "Установить", wsTabStop|bsDefPush, 242, 304, 110, 32, idcOK)
	add("BUTTON", "Отмена", wsTabStop, 360, 304, 86, 32, idcCancel)
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		id := wParam & 0xFFFF
		switch id {
		case idcBrowse:
			if dir := browseFolder(hwnd, windowText(child(hwnd, idcDir))); dir != "" {
				_, _, _ = procSetWindowText.Call(child(hwnd, idcDir), uintptr(unsafe.Pointer(wide(dir))))
			}
		case idcOK:
			doInstall(hwnd)
		case idcCancel:
			ui.cancelled = true
			_, _, _ = procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmClose:
		ui.cancelled = true
		_, _, _ = procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		_, _, _ = procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return r
}

func doInstall(hwnd uintptr) {
	_, _, _ = procEnableWindow.Call(child(hwnd, idcOK), 0)
	opt := ui.opt
	opt.Dir = windowText(child(hwnd, idcDir))
	opt.StartMenu = isChecked(child(hwnd, idcStart))
	opt.Desktop = isChecked(child(hwnd, idcDesk))
	opt.Associations = isChecked(child(hwnd, idcAssoc))
	res, err := install.Install(opt)
	if err != nil {
		_, _, _ = procEnableWindow.Call(child(hwnd, idcOK), 1)
		messageBox(hwnd, err.Error(), "boo", mbOK|mbIconErr)
		return
	}
	ui.res = res
	text := "boo установлен в\n" + res.Dir + "\n\nКниги и заметки при удалении программы не стираются.\n\nЗапустить сейчас?"
	if messageBox(hwnd, text, "boo", mbYesNo|mbIconInfo) == idYes {
		_ = exec.Command(res.Exe).Start()
	}
	_, _, _ = procDestroyWindow.Call(hwnd)
}

func child(parent uintptr, id uintptr) uintptr {
	h, _, _ := procGetDlgItem.Call(parent, id)
	return h
}

func isChecked(hwnd uintptr) bool {
	r, _, _ := procSendMessage.Call(hwnd, bmGetCheck, 0, 0)
	return r == bstChecked
}

func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLen.Call(hwnd)
	buf := make([]uint16, n+2)
	_, _, _ = procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf)
}

func wide(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func messageBox(hwnd uintptr, text, caption string, flags uint32) int {
	r, _ := windows.MessageBox(windows.HWND(hwnd), wide(text), wide(caption), flags)
	return int(r)
}

func browseFolder(owner uintptr, current string) string {
	buf := make([]uint16, 260)
	bi := browseInfo{
		hwndOwner:      owner,
		pszDisplayName: &buf[0],
		lpszTitle:      wide("Каталог установки boo"),
		ulFlags:        0x0001 | 0x0040,
	}
	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return current
	}
	path := make([]uint16, 260)
	ok, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	_, _, _ = procCoTaskMemFree.Call(pidl)
	if ok == 0 {
		return current
	}
	got := windows.UTF16ToString(path)
	if got == "" {
		return current
	}
	return got
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type rect struct {
	left, top, right, bottom int32
}

type nativeMsg struct {
	hwnd    uintptr
	message uint32
	pad     uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	_              uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
	_              uint32
}
