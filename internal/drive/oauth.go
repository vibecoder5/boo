package drive

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type credentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type pendingAuth struct {
	state     string
	verifier  string
	redirect  string
	createdAt time.Time
}

func (s *Service) oauthPath() string {
	return filepath.Join(s.driveDir(), "oauth.json")
}

func (s *Service) tokenPath() string {
	return filepath.Join(s.driveDir(), "token.json")
}

func (s *Service) HasCredentials() bool {
	c, err := s.loadCredentials()
	return err == nil && c.ClientID != "" && c.ClientSecret != ""
}

func (s *Service) SaveCredentials(clientID, clientSecret string) error {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("нужны идентификатор и секрет клиента Google")
	}
	if err := os.MkdirAll(s.driveDir(), 0o700); err != nil {
		return fmt.Errorf("не удалось сохранить ключ")
	}
	raw, err := json.MarshalIndent(credentials{ClientID: clientID, ClientSecret: clientSecret}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.oauthPath(), raw, 0o600); err != nil {
		return fmt.Errorf("не удалось сохранить ключ")
	}
	return nil
}

func (s *Service) loadCredentials() (credentials, error) {
	raw, err := os.ReadFile(s.oauthPath())
	if err != nil {
		return credentials{}, err
	}
	var c credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return credentials{}, fmt.Errorf("не удалось прочитать ключ Google")
	}
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	if c.ClientID == "" || c.ClientSecret == "" {
		return credentials{}, fmt.Errorf("ключ Google неполный")
	}
	return c, nil
}

func (s *Service) oauthConfig(redirect string) (*oauth2.Config, error) {
	c, err := s.loadCredentials()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("сначала сохраните ключ приложения Google")
		}
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  redirect,
		Scopes:       []string{"https://www.googleapis.com/auth/drive.file"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}, nil
}

func (s *Service) StartConnect(redirectURL string) (string, error) {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return "", fmt.Errorf("нет адреса для входа в Google")
	}
	cfg, err := s.oauthConfig(redirectURL)
	if err != nil {
		return "", err
	}
	state, err := randomID()
	if err != nil {
		return "", fmt.Errorf("не удалось начать вход")
	}
	verifier := oauth2.GenerateVerifier()
	s.mu.Lock()
	s.pending = &pendingAuth{state: state, verifier: verifier, redirect: redirectURL, createdAt: time.Now()}
	s.mu.Unlock()
	url := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier),
	)
	if s.openURL != nil {
		_ = s.openURL(url)
	}
	return url, nil
}

func (s *Service) FinishConnect(ctx context.Context, state, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("Google не вернул код входа")
	}
	s.mu.Lock()
	p := s.pending
	s.mu.Unlock()
	if p == nil || p.state == "" || state != p.state {
		return fmt.Errorf("вход в Google устарел, попробуйте ещё раз")
	}
	if time.Since(p.createdAt) > 15*time.Minute {
		return fmt.Errorf("вход в Google устарел, попробуйте ещё раз")
	}
	cfg, err := s.oauthConfig(p.redirect)
	if err != nil {
		return err
	}
	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(p.verifier))
	if err != nil {
		return fmt.Errorf("не удалось войти в Google")
	}
	if err := s.saveToken(tok); err != nil {
		return err
	}
	s.mu.Lock()
	s.pending = nil
	s.mu.Unlock()
	saved := s.loadPersisted()
	saved.LastError = ""
	if email, err := s.fetchEmail(ctx); err == nil {
		saved.Email = email
	}
	s.savePersisted(saved)
	return nil
}

func (s *Service) Disconnect() error {
	s.mu.Lock()
	s.pending = nil
	s.mu.Unlock()
	_ = os.Remove(s.tokenPath())
	saved := s.loadPersisted()
	saved.Email = ""
	saved.LastError = ""
	s.savePersisted(saved)
	return nil
}

func (s *Service) loadToken() (*oauth2.Token, error) {
	raw, err := os.ReadFile(s.tokenPath())
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("не удалось прочитать вход в Google")
	}
	return &tok, nil
}

func (s *Service) saveToken(tok *oauth2.Token) error {
	if tok == nil {
		return fmt.Errorf("Google Диск не подключён")
	}
	if err := os.MkdirAll(s.driveDir(), 0o700); err != nil {
		return fmt.Errorf("не удалось сохранить вход в Google")
	}
	raw, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.tokenPath(), raw, 0o600); err != nil {
		return fmt.Errorf("не удалось сохранить вход в Google")
	}
	return nil
}

func (s *Service) httpClient(ctx context.Context) (*http.Client, error) {
	tok, err := s.loadToken()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Google Диск не подключён")
		}
		return nil, err
	}
	cfg, err := s.oauthConfig("")
	if err != nil {
		return nil, err
	}
	base := cfg.TokenSource(ctx, tok)
	src := &savingTokenSource{src: oauth2.ReuseTokenSource(tok, base), save: s.saveToken}
	return oauth2.NewClient(ctx, src), nil
}

type savingTokenSource struct {
	src  oauth2.TokenSource
	save func(*oauth2.Token) error
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		return nil, fmt.Errorf("войдите в Google Диск снова")
	}
	_ = s.save(tok)
	return tok, nil
}

func (s *Service) fetchEmail(ctx context.Context) (string, error) {
	b, err := s.getBackend(ctx)
	if err != nil {
		return "", err
	}
	return b.About(ctx)
}

func randomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
