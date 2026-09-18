package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"boo/internal/store"
)

func (s *Server) handleDriveStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.drive.Status())
}

func (s *Server) handleDriveCredentials(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "нужны идентификатор и секрет клиента Google", http.StatusBadRequest)
		return
	}
	if err := s.drive.SaveCredentials(body.ClientID, body.ClientSecret); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, s.drive.Status())
}

func (s *Server) handleDriveConnect(w http.ResponseWriter, r *http.Request) {
	_, port, err := net.SplitHostPort(r.Host)
	if err != nil || port == "" {
		port = "7474"
	}
	redirect := "http://127.0.0.1:" + port + "/api/drive/callback"
	authURL, err := s.drive.StartConnect(redirect)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"url": authURL})
}

func (s *Server) handleDriveCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if msg := strings.TrimSpace(r.URL.Query().Get("error")); msg != "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, driveCallbackPage, "Не удалось войти в Google. Можно закрыть это окно и попробовать снова в boo.")
		return
	}
	if err := s.drive.FinishConnect(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code")); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, driveCallbackPage, html.EscapeString(err.Error())+". Можно закрыть это окно и вернуться в boo.")
		return
	}
	_, _ = fmt.Fprintf(w, driveCallbackPage, "Google Диск подключён. Можно закрыть это окно и вернуться в boo.")
}

func (s *Server) handleDriveSync(w http.ResponseWriter, r *http.Request) {
	if _, err := s.drive.Sync(r.Context(), s.store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.refreshAfterDrive()
	s.handleState(w, r)
}

func (s *Server) handleDriveDisconnect(w http.ResponseWriter, r *http.Request) {
	_ = s.drive.Disconnect()
	s.handleState(w, r)
}

func (s *Server) refreshAfterDrive() {
	s.dropAllDicts()
	book := s.current()
	if book == nil {
		return
	}
	if store.BundledFormat(bookFormat(book)) {
		return
	}
	if _, ok := s.store.EntryAnywhere(book.Key); !ok {
		s.setBook(nil)
	}
}

func (s *Server) SyncDriveOnClose() {
	if s.drive == nil || !s.drive.Connected() {
		return
	}
	fmt.Fprintln(os.Stderr, "boo: синхронизация с Google Диском…")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if _, err := s.drive.Sync(ctx, s.store); err != nil {
		fmt.Fprintf(os.Stderr, "boo: синхронизация с Google Диском: %v\n", err)
	}
}

const driveCallbackPage = `<!DOCTYPE html>
<html lang="ru">
<head><meta charset="utf-8"><title>boo</title></head>
<body style="font-family:system-ui,sans-serif;padding:2rem;line-height:1.5">
<p>%s</p>
</body>
</html>
`
