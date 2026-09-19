//go:build windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	uninstKeyRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	fileClasses   = `Software\Classes`
	webviewGUID   = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
)

var webviewKeys = []struct {
	root registry.Key
	path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + webviewGUID},
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webviewGUID},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webviewGUID},
}

// WebView2Installed смотрит только реестр, в сеть не ходит.
func WebView2Installed() bool {
	for _, k := range webviewKeys {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		ver, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && strings.TrimSpace(ver) != "" && ver != "0.0.0.0" {
			return true
		}
	}
	return false
}

// Install копирует файлы, пишет реестр и ярлыки. Каталог данных не создаёт.
func Install(opt Options) (Result, error) {
	opt, err := opt.normalized()
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(opt.Dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("не удалось создать каталог: %w", err)
	}

	exe := exePath(opt.Dir, opt.Profile.ExeName)
	uninst := filepath.Join(opt.Dir, "uninstall.exe")
	if err := writeExe(exe, opt.Bundle.App); err != nil {
		return Result{}, err
	}
	uninstBytes := opt.Bundle.Uninstaller
	if len(uninstBytes) == 0 {
		uninstBytes = opt.Bundle.App
	}
	if err := writeExe(uninst, uninstBytes); err != nil {
		return Result{}, err
	}

	startMenu := ""
	if opt.StartMenu {
		dir := opt.StartMenuDir
		if dir == "" {
			dir = defaultStartMenuDir(opt.Profile.Name)
		}
		startMenu = filepath.Join(dir, opt.Profile.Name+".lnk")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Result{}, fmt.Errorf("меню «Пуск»: %w", err)
		}
		if err := createShortcut(startMenu, exe, "", opt.Dir, opt.Profile.Name); err != nil {
			return Result{}, fmt.Errorf("ярлык в меню «Пуск»: %w", err)
		}
	}

	desktop := ""
	if opt.Desktop {
		dir := opt.DesktopDir
		if dir == "" {
			dir = defaultDesktopDir()
		}
		desktop = filepath.Join(dir, opt.Profile.Name+".lnk")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Result{}, fmt.Errorf("рабочий стол: %w", err)
		}
		if err := createShortcut(desktop, exe, "", opt.Dir, opt.Profile.Name); err != nil {
			return Result{}, fmt.Errorf("ярлык на рабочем столе: %w", err)
		}
	}

	if opt.Associations {
		if err := registerAssociations(opt.Profile, exe); err != nil {
			return Result{}, err
		}
	}

	if err := writeUninstallKey(opt, exe, uninst, startMenu, desktop); err != nil {
		return Result{}, err
	}
	notifyAssocChanged()
	return Result{Dir: opt.Dir, Exe: exe, Uninstaller: uninst}, nil
}

// Uninstall снимает программу. %AppData%\boo не удаляет.
func Uninstall(profile Profile) error {
	profile = profile.withDefaults()
	info, err := readUninstallKey(profile)
	if err != nil {
		return err
	}
	if info.StartMenu != "" {
		_ = os.Remove(info.StartMenu)
		_ = os.Remove(filepath.Dir(info.StartMenu))
	}
	if info.Desktop != "" {
		_ = os.Remove(info.Desktop)
	}
	if info.Associations {
		removeAssociations(profile)
	}
	if info.Dir != "" {
		_ = os.Remove(filepath.Join(info.Dir, profile.ExeName))
		_ = os.Remove(filepath.Join(info.Dir, "uninstall.exe"))
		_ = os.Remove(info.Dir)
	}
	if err := deleteUninstallKey(profile); err != nil {
		return err
	}
	notifyAssocChanged()
	return nil
}

// Installed сообщает, есть ли запись в «Программах и компонентах».
func Installed(profile Profile) (dir string, ok bool) {
	info, err := readUninstallKey(profile.withDefaults())
	if err != nil || info.Dir == "" {
		return "", false
	}
	return info.Dir, true
}

func writeExe(path string, data []byte) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("не удалось записать %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("закройте boo и повторите установку: %w", err)
		}
	}
	return nil
}

type uninstallInfo struct {
	Dir          string
	StartMenu    string
	Desktop      string
	Associations bool
}

func uninstallKeyPath(id string) string {
	return uninstKeyRoot + `\` + id
}

func writeUninstallKey(opt Options, exe, uninst, startMenu, desktop string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath(opt.Profile.ID), registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("реестр удаления: %w", err)
	}
	defer key.Close()
	sizeKB := (len(opt.Bundle.App) + len(opt.Bundle.Uninstaller)) / 1024
	vals := map[string]string{
		"DisplayName":     opt.Profile.Name,
		"DisplayVersion":  opt.Profile.Version,
		"Publisher":       "boo",
		"InstallLocation": opt.Dir,
		"DisplayIcon":     exe,
		"UninstallString": `"` + uninst + `" -id ` + opt.Profile.ID,
		"StartMenu":       startMenu,
		"DesktopShortcut": desktop,
	}
	for name, val := range vals {
		if err := key.SetStringValue(name, val); err != nil {
			return fmt.Errorf("реестр удаления: %w", err)
		}
	}
	if err := key.SetDWordValue("NoModify", 1); err != nil {
		return err
	}
	if err := key.SetDWordValue("NoRepair", 1); err != nil {
		return err
	}
	if err := key.SetDWordValue("EstimatedSize", uint32(sizeKB)); err != nil {
		return err
	}
	assoc := uint32(0)
	if opt.Associations {
		assoc = 1
	}
	return key.SetDWordValue("Associations", assoc)
}

func readUninstallKey(profile Profile) (uninstallInfo, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath(profile.ID), registry.QUERY_VALUE)
	if err != nil {
		return uninstallInfo{}, fmt.Errorf("boo не установлен")
	}
	defer key.Close()
	var info uninstallInfo
	info.Dir, _, _ = key.GetStringValue("InstallLocation")
	info.StartMenu, _, _ = key.GetStringValue("StartMenu")
	info.Desktop, _, _ = key.GetStringValue("DesktopShortcut")
	if v, _, err := key.GetIntegerValue("Associations"); err == nil {
		info.Associations = v != 0
	}
	return info, nil
}

func deleteUninstallKey(profile Profile) error {
	err := registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath(profile.ID))
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("реестр удаления: %w", err)
	}
	return nil
}

func registerAssociations(p Profile, exe string) error {
	for _, ext := range []string{".epub", ".fb2"} {
		prog := p.progID(ext)
		if err := setClassDefault(ext, prog); err != nil {
			return err
		}
		if err := setClassDefault(prog, "Книга "+strings.ToUpper(strings.TrimPrefix(ext, "."))); err != nil {
			return err
		}
		if err := setClassDefault(prog+`\DefaultIcon`, exe+",0"); err != nil {
			return err
		}
		if err := setClassDefault(prog+`\shell\open\command`, `"`+exe+`" "%1"`); err != nil {
			return err
		}
		if err := setOpenWith(ext, prog); err != nil {
			return err
		}
	}
	return nil
}

func removeAssociations(p Profile) {
	for _, ext := range []string{".epub", ".fb2"} {
		prog := p.progID(ext)
		key, err := registry.OpenKey(registry.CURRENT_USER, fileClasses+`\`+ext, registry.QUERY_VALUE)
		if err == nil {
			cur, _, _ := key.GetStringValue("")
			key.Close()
			if strings.EqualFold(cur, prog) {
				_ = deleteClassTree(ext)
			} else {
				_ = registry.DeleteKey(registry.CURRENT_USER, fileClasses+`\`+ext+`\OpenWithProgids`)
			}
		}
		_ = deleteClassTree(prog)
	}
}

func setClassDefault(rel, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, fileClasses+`\`+rel, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("ассоциации файлов: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue("", value); err != nil {
		return fmt.Errorf("ассоциации файлов: %w", err)
	}
	return nil
}

func setOpenWith(ext, prog string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, fileClasses+`\`+ext+`\OpenWithProgids`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(prog, "")
}

func deleteClassTree(rel string) error {
	path := fileClasses + `\` + rel
	deleteRegistryTree(registry.CURRENT_USER, path)
	return nil
}

func deleteRegistryTree(root registry.Key, path string) {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return
	}
	names, _ := key.ReadSubKeyNames(-1)
	key.Close()
	for _, name := range names {
		deleteRegistryTree(root, path+`\`+name)
	}
	_ = registry.DeleteKey(root, path)
}

func defaultStartMenuDir(name string) string {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		home, _ := os.UserHomeDir()
		appdata = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appdata, `Microsoft\Windows\Start Menu\Programs`, name)
}

func defaultDesktopDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

var (
	ole32DLL             = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32DLL.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32DLL.NewProc("CoUninitialize")
	procCoCreateInstance = ole32DLL.NewProc("CoCreateInstance")
	clsidShellLink       = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellLinkW       = windows.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPersistFile      = windows.GUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

const (
	coinitApartment    = 0x2
	clsctxInprocServer = 0x1
	rpcEChangedMode    = 0x80010106
	shcneAssocChanged  = 0x08000000
	shcnfFlushNoWait   = 0x2000
)

func createShortcut(lnk, target, args, workdir, desc string) error {
	uninit, err := coInit()
	if err != nil {
		return fmt.Errorf("ярлык: %w", err)
	}
	if uninit {
		defer procCoUninitialize.Call()
	}

	var psl uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIShellLinkW)),
		uintptr(unsafe.Pointer(&psl)),
	)
	if err := hresult(hr); err != nil || psl == 0 {
		return fmt.Errorf("ярлык: %v", err)
	}
	defer comRelease(psl)

	if err := comSetString(psl, 20, target); err != nil {
		return fmt.Errorf("ярлык: путь: %w", err)
	}
	if err := comSetString(psl, 11, args); err != nil {
		return fmt.Errorf("ярлык: аргументы: %w", err)
	}
	if err := comSetString(psl, 9, workdir); err != nil {
		return fmt.Errorf("ярлык: каталог: %w", err)
	}
	if err := comSetString(psl, 7, desc); err != nil {
		return fmt.Errorf("ярлык: описание: %w", err)
	}
	icon, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("ярлык: %w", err)
	}
	if err := hresult(comCall(psl, 17, uintptr(unsafe.Pointer(icon)), 0)); err != nil {
		return fmt.Errorf("ярлык: значок: %w", err)
	}

	var ppf uintptr
	if err := hresult(comCall(psl, 0, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&ppf)))); err != nil || ppf == 0 {
		return fmt.Errorf("ярлык: IPersistFile: %v", err)
	}
	defer comRelease(ppf)

	path, err := windows.UTF16PtrFromString(lnk)
	if err != nil {
		return fmt.Errorf("ярлык: %w", err)
	}
	if err := hresult(comCall(ppf, 6, uintptr(unsafe.Pointer(path)), 1)); err != nil {
		return fmt.Errorf("не удалось сохранить ярлык: %w", err)
	}
	return nil
}

func coInit() (uninit bool, err error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartment)
	if hr == 0 {
		return true, nil
	}
	if hr == 1 || uint32(hr) == rpcEChangedMode {
		return false, nil
	}
	return false, fmt.Errorf("COM 0x%08X", uint32(hr))
}

func comCall(obj uintptr, idx int, args ...uintptr) uintptr {
	vtbl := *(**[32]uintptr)(unsafe.Pointer(obj))
	all := append([]uintptr{obj}, args...)
	hr, _, _ := syscall.SyscallN(vtbl[idx], all...)
	return hr
}

func comSetString(obj uintptr, idx int, s string) error {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return err
	}
	return hresult(comCall(obj, idx, uintptr(unsafe.Pointer(p))))
}

func comRelease(obj uintptr) {
	if obj == 0 {
		return
	}
	_ = comCall(obj, 2)
}

func hresult(hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("0x%08X", uint32(hr))
	}
	return nil
}

func notifyAssocChanged() {
	// SHCNF_FLUSHNOWAIT: не ждать ответа окон — иначе вызов из оконной процедуры зависает.
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	_, _, _ = shell32.NewProc("SHChangeNotify").Call(shcneAssocChanged, shcnfFlushNoWait, 0, 0)
}
