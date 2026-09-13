package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"boo/internal/install"
)

var quiet bool

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "boo: %v\n", err)
		if !quiet {
			alertError(err)
		}
		os.Exit(1)
	}
}

func run() error {
	silent := flag.Bool("silent", false, "поставить без вопросов")
	uninstall := flag.Bool("uninstall", false, "удалить программу")
	dir := flag.String("dir", "", "каталог установки")
	desktop := flag.Bool("desktop", false, "ярлык на рабочем столе")
	noStart := flag.Bool("no-startmenu", false, "без ярлыка в меню «Пуск»")
	noAssoc := flag.Bool("no-assoc", false, "не открывать EPUB и FB2 в boo")
	bin := flag.String("bin", "", "путь к boo.exe (сборка без вложения)")
	pack := flag.Bool("pack", false, "собрать setup.exe из -bin")
	out := flag.String("out", "", "путь для -pack")
	id := flag.String("id", "", "ключ в списке программ (по умолчанию boo)")
	flag.Parse()

	self, err := os.Executable()
	if err != nil {
		return err
	}
	if *pack {
		quiet = true
		return runPack(self, *bin, *out)
	}
	if *silent {
		quiet = true
	}
	if strings.EqualFold(filepath.Base(self), "uninstall.exe") {
		*uninstall = true
	}

	if *uninstall {
		return runUninstall(*silent, strings.TrimSpace(*id))
	}

	bundle, err := loadBundle(self, *bin)
	if err != nil {
		return err
	}
	opt := install.Options{
		Dir:          strings.TrimSpace(*dir),
		Bundle:       bundle,
		StartMenu:    !*noStart,
		Desktop:      *desktop,
		Associations: !*noAssoc,
		Profile:      install.Profile{ID: strings.TrimSpace(*id)},
	}
	if *silent {
		return runSilent(opt)
	}
	return runUI(opt)
}

func loadBundle(self, bin string) (install.Bundle, error) {
	raw, err := os.ReadFile(self)
	if err != nil {
		return install.Bundle{}, err
	}
	if b, err := install.Unpack(raw); err == nil {
		return b, nil
	}
	if strings.TrimSpace(bin) == "" {
		return install.Bundle{}, fmt.Errorf("нет вложения: укажите -bin путь\\boo.exe или соберите setup через scripts/build-installer.ps1")
	}
	app, err := os.ReadFile(bin)
	if err != nil {
		return install.Bundle{}, fmt.Errorf("не удалось прочитать %s: %w", bin, err)
	}
	uninst, err := os.ReadFile(self)
	if err != nil {
		return install.Bundle{}, err
	}
	return install.Bundle{App: app, Uninstaller: uninst}, nil
}

func runSilent(opt install.Options) error {
	res, err := install.Install(opt)
	if err != nil {
		return err
	}
	fmt.Printf("boo %s установлен в %s\n", install.Version, res.Dir)
	if !install.WebView2Installed() {
		fmt.Println("WebView2 не найден: окно может не открыться, тогда запустите с флагом -web.")
	}
	fmt.Println("Книги и заметки живут отдельно и при удалении программы не стираются.")
	return nil
}

func runPack(self, bin, out string) error {
	if strings.TrimSpace(bin) == "" || strings.TrimSpace(out) == "" {
		return fmt.Errorf("для -pack нужны -bin и -out")
	}
	host, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	app, err := os.ReadFile(bin)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s: %w", bin, err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	packed := install.Pack(host, install.Bundle{App: app, Uninstaller: host})
	if err := os.WriteFile(out, packed, 0o755); err != nil {
		return err
	}
	fmt.Printf("собрано %s (%d байт)\n", out, len(packed))
	return nil
}

func runUninstall(silent bool, id string) error {
	if !silent && !confirmUninstall() {
		return nil
	}
	if err := install.Uninstall(install.Profile{ID: id}); err != nil {
		return err
	}
	fmt.Println("boo удалён. Папка с книгами и заметками не тронута.")
	return nil
}
