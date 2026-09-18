package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"boo/internal/desktop"
	"boo/internal/epub"
	"boo/internal/open"
	"boo/internal/server"
	"boo/internal/store"
)

//go:embed all:web
var webFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "boo: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.Int("port", 7474, "порт локального сервера")
	web := flag.Bool("web", false, "открыть в браузере вместо окна")
	noOpen := flag.Bool("no-open", false, "только сервер, без окна и браузера")
	demo := flag.Bool("demo", false, "сразу открыть демо-книгу")
	flag.Parse()

	st, err := store.Open()
	if err != nil {
		return err
	}

	var book *epub.Book
	if *demo {
		data, err := epub.Sample()
		if err != nil {
			return err
		}
		book, err = epub.OpenBytes("demo.epub", data)
		if err != nil {
			return err
		}
		defer book.Close()
	} else if path := flag.Arg(0); path != "" {
		book, err = open.Open(path)
		if err != nil {
			return err
		}
		defer book.Close()
	} else if last := server.OpenLast(st); last != nil {
		book = last
		defer book.Close()
	}

	ui, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}

	srv := server.New(st, ui, book)
	ln, err := srv.Listen(fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return err
	}
	appURL := "http://" + ln.Addr().String() + "/"
	fmt.Printf("boo: %s\n", appURL)
	if book != nil {
		fmt.Printf("книга: %s\n", book.Title)
	}

	handler := srv.Handler()
	useWindow := !*web && !*noOpen && desktop.Supported()

	if useWindow {
		serveErr := make(chan error, 1)
		go func() {
			serveErr <- http.Serve(ln, handler)
		}()
		if err := waitReady(appURL); err != nil {
			return err
		}
		title := ""
		if book != nil {
			title = book.Title
		}
		if err := desktop.Run(appURL, title); err != nil {
			fmt.Fprintf(os.Stderr, "boo: окно недоступно (%v), открываю браузер\n", err)
			_ = openBrowser(appURL)
			return <-serveErr
		}
		srv.RecordSession()
		srv.SyncDriveOnClose()
		return nil
	}

	if !*noOpen {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = openBrowser(appURL)
		}()
	}
	return http.Serve(ln, handler)
}

func waitReady(appURL string) error {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(appURL)
		if err == nil {
			res.Body.Close()
			if res.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(80 * time.Millisecond)
	}
	return fmt.Errorf("сервер не запустился")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
