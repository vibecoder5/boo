//go:build !windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"boo/internal/install"
)

func alertError(error) {}

func runUI(opt install.Options) error {
	dir := opt.Dir
	if dir == "" {
		d, err := install.DefaultDir()
		if err != nil {
			return err
		}
		dir = d
	}
	fmt.Printf("boo %s — установка\n", install.Version)
	fmt.Printf("Каталог: %s\n", dir)
	fmt.Print("Enter — установить, Q — выход: ")
	in := bufio.NewReader(os.Stdin)
	line, _ := in.ReadString('\n')
	if strings.EqualFold(strings.TrimSpace(line), "q") {
		return nil
	}
	opt.Dir = dir
	return runSilent(opt)
}

func confirmUninstall() bool {
	fmt.Print("Удалить boo? Книги в профиле останутся. [Y/N]: ")
	in := bufio.NewReader(os.Stdin)
	line, _ := in.ReadString('\n')
	s := strings.TrimSpace(strings.ToLower(line))
	return s == "y" || s == "д" || s == "yes" || s == "да"
}
