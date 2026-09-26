//go:build windows

package desktop

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clsctxInprocServer = 0x1
	fosPickFolders     = 0x20
	fosForceFilesystem = 0x40
	fosPathMustExist   = 0x800
	sigdnFileSysPath   = 0x80058000
	hresultCancelled   = 0x800704C7
	rpcEChangedMode    = 0x80010106
)

var (
	pickMu       sync.Mutex
	ownerHWND    uintptr
	ole32        = windows.NewLazySystemDLL("ole32.dll")
	procCoCreate = ole32.NewProc("CoCreateInstance")

	clsidFileOpenDialog = windows.GUID{
		Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE,
		Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
	}
	iidIFileOpenDialog = windows.GUID{
		Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768,
		Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
	}
)

func noteWindow(hwnd unsafe.Pointer) {
	if hwnd == nil {
		atomic.StoreUintptr(&ownerHWND, 0)
		return
	}
	atomic.StoreUintptr(&ownerHWND, uintptr(hwnd))
}

func CanPickFolder() bool { return true }

type fileDialogVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	Show                uintptr
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
}

type fileDialog struct {
	lpVtbl *fileDialogVtbl
}

type shellItemVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

type shellItem struct {
	lpVtbl *shellItemVtbl
}

func PickFolder(title string) (string, bool, error) {
	pickMu.Lock()
	defer pickMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	uninit, err := initCOM()
	if err != nil {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	if uninit {
		defer windows.CoUninitialize()
	}

	var punk unsafe.Pointer
	hr, _, _ := procCoCreate.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&punk)),
	)
	if hrFailed(hr) || punk == nil {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	dlg := (*fileDialog)(punk)
	defer syscall.SyscallN(dlg.lpVtbl.Release, uintptr(unsafe.Pointer(dlg)))

	var opts uint32
	hr, _, _ = syscall.SyscallN(dlg.lpVtbl.GetOptions, uintptr(unsafe.Pointer(dlg)), uintptr(unsafe.Pointer(&opts)))
	if hrFailed(hr) {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	opts |= fosPickFolders | fosForceFilesystem | fosPathMustExist
	hr, _, _ = syscall.SyscallN(dlg.lpVtbl.SetOptions, uintptr(unsafe.Pointer(dlg)), uintptr(opts))
	if hrFailed(hr) {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	if title != "" {
		ptr, err := windows.UTF16PtrFromString(title)
		if err == nil {
			_, _, _ = syscall.SyscallN(dlg.lpVtbl.SetTitle, uintptr(unsafe.Pointer(dlg)), uintptr(unsafe.Pointer(ptr)))
		}
	}
	owner := atomic.LoadUintptr(&ownerHWND)
	hr, _, _ = syscall.SyscallN(dlg.lpVtbl.Show, uintptr(unsafe.Pointer(dlg)), owner)
	if hr == hresultCancelled {
		return "", false, nil
	}
	if hrFailed(hr) {
		return "", false, errors.New("не удалось открыть выбор папки")
	}

	var itemPtr unsafe.Pointer
	hr, _, _ = syscall.SyscallN(dlg.lpVtbl.GetResult, uintptr(unsafe.Pointer(dlg)), uintptr(unsafe.Pointer(&itemPtr)))
	if hrFailed(hr) || itemPtr == nil {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	item := (*shellItem)(itemPtr)
	defer syscall.SyscallN(item.lpVtbl.Release, uintptr(unsafe.Pointer(item)))

	var name *uint16
	hr, _, _ = syscall.SyscallN(
		item.lpVtbl.GetDisplayName,
		uintptr(unsafe.Pointer(item)),
		sigdnFileSysPath,
		uintptr(unsafe.Pointer(&name)),
	)
	if hrFailed(hr) || name == nil {
		return "", false, errors.New("не удалось открыть выбор папки")
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name), true, nil
}

func initCOM() (bool, error) {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil {
		return true, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	if errno == 1 {
		return true, nil
	}
	if uint32(errno) == rpcEChangedMode {
		return false, nil
	}
	return false, err
}

func hrFailed(hr uintptr) bool {
	return int32(hr) < 0
}
