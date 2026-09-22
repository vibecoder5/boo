package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Version попадает в «Программы и компоненты». Канон номера — CHANGELOG.md.
var Version = "0.9.2"

// Options — что поставить и куда.
type Options struct {
	Dir          string
	Bundle       Bundle
	StartMenu    bool
	Desktop      bool
	Associations bool
	Profile      Profile

	// Каталоги ярлыков; пустые — стандартные папки пользователя.
	StartMenuDir string
	DesktopDir   string
}

// Profile отличает боевую установку от тестовой (другие ключи реестра).
type Profile struct {
	ID      string
	Name    string
	Version string
	ExeName string
}

func (p Profile) withDefaults() Profile {
	if p.ID == "" {
		p.ID = "boo"
	}
	if p.Name == "" {
		p.Name = "boo"
	}
	if p.Version == "" {
		p.Version = Version
	}
	if p.ExeName == "" {
		p.ExeName = "boo.exe"
	}
	return p
}

func (p Profile) progID(ext string) string {
	return p.ID + "." + strings.TrimPrefix(ext, ".")
}

// Result — куда легли файлы.
type Result struct {
	Dir         string
	Exe         string
	Uninstaller string
}

// DefaultDir — %LocalAppData%\Programs\boo, без прав администратора.
func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.Getenv("LOCALAPPDATA")
	}
	if strings.TrimSpace(base) == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("не удалось определить каталог установки: %v", err)
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "Programs", "boo"), nil
}

// DataDir — библиотека и настройки; деинсталлятор сюда не заходит.
func DataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "boo")
}

func (o Options) normalized() (Options, error) {
	o.Profile = o.Profile.withDefaults()
	if strings.TrimSpace(o.Dir) == "" {
		dir, err := DefaultDir()
		if err != nil {
			return o, err
		}
		o.Dir = dir
	}
	o.Dir = filepath.Clean(o.Dir)
	if len(o.Bundle.App) == 0 {
		return o, fmt.Errorf("нет файла программы")
	}
	return o, nil
}

func exePath(dir, name string) string {
	return filepath.Join(dir, name)
}
