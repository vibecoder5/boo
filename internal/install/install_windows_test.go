//go:build windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func testProfile(t *testing.T) Profile {
	t.Helper()
	return Profile{
		ID:      "boo-test-installer",
		Name:    "boo-test-installer",
		Version: "0.0.0-test",
		ExeName: "boo.exe",
	}
}

func TestInstallUninstall(t *testing.T) {
	p := testProfile(t)
	_ = Uninstall(p)

	root := t.TempDir()
	dir := filepath.Join(root, "app")
	start := filepath.Join(root, "start")
	desk := filepath.Join(root, "desk")
	dataProbe := filepath.Join(root, "should-keep")
	if err := os.WriteFile(dataProbe, []byte("library"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Install(Options{
		Dir:          dir,
		Bundle:       Bundle{App: []byte("fake-boo"), Uninstaller: []byte("fake-uninst")},
		StartMenu:    true,
		Desktop:      true,
		Associations: true,
		Profile:      p,
		StartMenuDir: start,
		DesktopDir:   desk,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Exe == "" {
		t.Fatal("empty exe")
	}
	if _, err := os.Stat(res.Exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Uninstaller); err != nil {
		t.Fatal(err)
	}
	if err := checkShortcut(filepath.Join(start, p.Name+".lnk")); err != nil {
		t.Fatalf("start menu: %v", err)
	}
	if err := checkShortcut(filepath.Join(desk, p.Name+".lnk")); err != nil {
		t.Fatalf("desktop: %v", err)
	}

	got, ok := Installed(p)
	if !ok || got != dir {
		t.Fatalf("Installed: dir=%q ok=%v", got, ok)
	}
	ukey, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath(p.ID), registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	cmd, _, err := ukey.GetStringValue("UninstallString")
	ukey.Close()
	if err != nil || !strings.Contains(cmd, "-id "+p.ID) {
		t.Fatalf("UninstallString %q %v", cmd, err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, fileClasses+`\.epub`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := key.GetStringValue("")
	key.Close()
	if err != nil || prog != p.progID(".epub") {
		t.Fatalf("epub progid: %q %v", prog, err)
	}

	if err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := Installed(p); ok {
		t.Fatal("still installed")
	}
	if _, err := os.Stat(res.Exe); !os.IsNotExist(err) {
		t.Fatalf("exe left: %v", err)
	}
	if _, err := os.Stat(dataProbe); err != nil {
		t.Fatal("деинсталлятор не должен трогать посторонние файлы")
	}

	key, err = registry.OpenKey(registry.CURRENT_USER, fileClasses+`\`+p.progID(".epub"), registry.QUERY_VALUE)
	if err == nil {
		key.Close()
		t.Fatal("progid остался")
	}
}

func checkShortcut(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 4 || data[0] != 0x4C || data[1] != 0 || data[2] != 0 || data[3] != 0 {
		return fmt.Errorf("не ярлык Windows: %s", path)
	}
	return nil
}

func TestUninstallMissing(t *testing.T) {
	err := Uninstall(Profile{ID: "boo-test-installer-missing"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallRequiresApp(t *testing.T) {
	_, err := Install(Options{Dir: t.TempDir(), Profile: testProfile(t)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "boo" {
		t.Fatalf("dir=%q", dir)
	}
}

// Как в окне установщика: цикл сообщений крутится, установка идёт в другом потоке.
func TestInstallWhileWindowPumps(t *testing.T) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procCreateWindowEx := user32.NewProc("CreateWindowExW")
	procDefWindowProc := user32.NewProc("DefWindowProcW")
	procRegisterClassEx := user32.NewProc("RegisterClassExW")
	procGetMessage := user32.NewProc("GetMessageW")
	procDispatchMessage := user32.NewProc("DispatchMessageW")
	procPostQuitMessage := user32.NewProc("PostQuitMessage")
	procPostMessage := user32.NewProc("PostMessageW")
	procDestroyWindow := user32.NewProc("DestroyWindow")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)

	done := make(chan error, 2)
	className, _ := windows.UTF16PtrFromString("booInstallPumpTest")
	type wndClassEx struct {
		cbSize, style uint32
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
	type nativeMsg struct {
		hwnd    uintptr
		message uint32
		pad     uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
	}
	const wmDestroy = 0x0002
	const wmAppDone = 0x8001

	wndProc := func(h, msg, wParam, lParam uintptr) uintptr {
		if msg == wmDestroy {
			_, _, _ = procPostQuitMessage.Call(0)
			return 0
		}
		if msg == wmAppDone {
			_, _, _ = procDestroyWindow.Call(h)
			return 0
		}
		r, _, _ := procDefWindowProc.Call(h, msg, wParam, lParam)
		return r
	}
	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(wndProc),
		hInstance:     windows.Handle(instance),
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		t.Fatalf("RegisterClassEx: %v", err)
	}
	hwnd, _, err := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, 0, 0, instance, 0,
	)
	if hwnd == 0 {
		t.Fatalf("CreateWindowEx: %v", err)
	}

	p := testProfile(t)
	p.ID = "boo-test-install-pump"
	p.Name = p.ID
	_ = Uninstall(p)
	t.Cleanup(func() { _ = Uninstall(p) })

	root := t.TempDir()
	go func() {
		_, ierr := Install(Options{
			Dir:          filepath.Join(root, "app"),
			Bundle:       Bundle{App: []byte("fake-boo"), Uninstaller: []byte("fake-uninst")},
			StartMenu:    true,
			Desktop:      true,
			Associations: true,
			Profile:      p,
			StartMenuDir: filepath.Join(root, "start"),
			DesktopDir:   filepath.Join(root, "desk"),
		})
		done <- ierr
		_, _, _ = procPostMessage.Call(hwnd, wmAppDone, 0, 0)
	}()
	go func() {
		time.Sleep(15 * time.Second)
		select {
		case done <- fmt.Errorf("установка зависла, пока открыто окно"):
			_, _, _ = procPostMessage.Call(hwnd, wmAppDone, 0, 0)
		default:
		}
	}()

	var msg nativeMsg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		_, _, _ = procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
