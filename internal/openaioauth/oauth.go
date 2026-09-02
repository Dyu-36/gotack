package openaioauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	DefaultAuthURL = "https://auth.openai.com/oauth/authorize"

	DefaultTokenURL = "https://auth.openai.com/oauth/token"

	DefaultRedirectPort = 1455

	DefaultScopes = "openid profile email offline_access"
)

type Token struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	IDToken         string `json:"id_token,omitempty"`
	TokenType       string `json:"token_type,omitempty"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
	AccountID       string `json:"account_id,omitempty"`
	AccountEmail    string `json:"account_email,omitempty"`
	AccountPlan     string `json:"account_plan,omitempty"`
	ChatGPTUserID   string `json:"chatgpt_user_id,omitempty"`
	AccountFedRAMP  bool   `json:"chatgpt_account_is_fedramp,omitempty"`
	Email           string `json:"email,omitempty"`
	ChatGPTPlanType string `json:"chatgpt_plan_type,omitempty"`
}

func (t *Token) UserEmail() string {
	if t.AccountEmail != "" {
		return t.AccountEmail
	}
	return t.Email
}

func (t *Token) UserPlan() string {
	if t.AccountPlan != "" {
		return t.AccountPlan
	}
	return t.ChatGPTPlanType
}

func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type IDTokenClaims struct {
	Email          string
	Plan           string
	AccountID      string
	ChatGPTUserID  string
	AccountFedRAMP bool
}

func ParseIDTokenMetadata(idToken string) IDTokenClaims {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 || parts[1] == "" {
		return IDTokenClaims{}
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return IDTokenClaims{}
	}
	var claims struct {
		Email   string `json:"email"`
		Profile struct {
			Email string `json:"email"`
		} `json:"https://api.openai.com/profile"`
		Auth struct {
			PlanType       string `json:"chatgpt_plan_type"`
			UserID         string `json:"chatgpt_user_id"`
			LegacyUserID   string `json:"user_id"`
			AccountID      string `json:"chatgpt_account_id"`
			AccountFedRAMP bool   `json:"chatgpt_account_is_fedramp"`
		} `json:"https://api.openai.com/auth"`

		PlanType string `json:"chatgpt_plan_type"`
		Plan     string `json:"plan"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return IDTokenClaims{}
	}
	metadata := IDTokenClaims{
		Email:          claims.Email,
		Plan:           claims.Auth.PlanType,
		AccountID:      claims.Auth.AccountID,
		ChatGPTUserID:  claims.Auth.UserID,
		AccountFedRAMP: claims.Auth.AccountFedRAMP,
	}
	if metadata.Email == "" {
		metadata.Email = claims.Profile.Email
	}
	if metadata.Plan == "" {
		metadata.Plan = claims.PlanType
	}
	if metadata.Plan == "" {
		metadata.Plan = claims.Plan
	}
	if metadata.ChatGPTUserID == "" {
		metadata.ChatGPTUserID = claims.Auth.LegacyUserID
	}
	return metadata
}

func ParseIDTokenClaims(idToken string) (email, plan string) {
	metadata := ParseIDTokenMetadata(idToken)
	return metadata.Email, metadata.Plan
}

type Options struct {
	ClientID     string
	AuthURL      string
	TokenURL     string
	Port         int
	HTTPClient   *http.Client
	OpenBrowser  func(authURL string) error
	LoginTimeout time.Duration
}

func DefaultOptions() Options {
	return Options{
		ClientID:     DefaultClientID,
		AuthURL:      DefaultAuthURL,
		TokenURL:     DefaultTokenURL,
		Port:         DefaultRedirectPort,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		LoginTimeout: 3 * time.Minute,
	}
}

func StartLogin(ctx context.Context, opts Options) (*Token, error) {
	if opts.ClientID == "" {
		opts.ClientID = DefaultClientID
	}
	if opts.AuthURL == "" {
		opts.AuthURL = DefaultAuthURL
	}
	if opts.TokenURL == "" {
		opts.TokenURL = DefaultTokenURL
	}
	if opts.Port == 0 {
		opts.Port = DefaultRedirectPort
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.LoginTimeout == 0 {
		opts.LoginTimeout = 3 * time.Minute
	}

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}

	state, err := GenerateState()
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("start local OAuth callback listener on %s: %w", addr, err)
	}
	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", actualPort)

	vals := url.Values{}
	vals.Set("client_id", opts.ClientID)
	vals.Set("response_type", "code")
	vals.Set("redirect_uri", redirectURI)
	vals.Set("scope", DefaultScopes)
	vals.Set("code_challenge", challenge)
	vals.Set("code_challenge_method", "S256")
	vals.Set("state", state)
	vals.Set("id_token_add_organizations", "true")
	vals.Set("codex_cli_simplified_flow", "true")
	vals.Set("originator", "gotack")

	authURL := opts.AuthURL + "?" + vals.Encode()

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			desc := q.Get("error_description")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Gotack - Đăng nhập thất bại</title><style>body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#f8fafc}.card{background:#1e293b;padding:2.5rem;border-radius:1rem;text-align:center;box-shadow:0 10px 25px rgba(0,0,0,0.5);max-width:400px}h1{color:#ef4444;font-size:1.5rem;margin-bottom:0.75rem}p{color:#94a3b8;font-size:0.95rem;line-height:1.5}</style></head><body><div class="card"><h1>Đăng nhập không thành công</h1><p>%s: %s</p></div></body></html>`, htmlEscape(errMsg), htmlEscape(desc))))
			errChan <- fmt.Errorf("oauth error from provider: %s (%s)", errMsg, desc)
			return
		}

		receivedState := q.Get("state")
		if receivedState != state {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Gotack - Lỗi xác thực</title><style>body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#f8fafc}.card{background:#1e293b;padding:2.5rem;border-radius:1rem;text-align:center;box-shadow:0 10px 25px rgba(0,0,0,0.5);max-width:400px}h1{color:#ef4444;font-size:1.5rem;margin-bottom:0.75rem}p{color:#94a3b8;font-size:0.95rem;line-height:1.5}</style></head><body><div class="card"><h1>Lỗi xác thực</h1><p>State không hợp lệ. Vui lòng thử lại.</p></div></body></html>`))
			errChan <- errors.New("state mismatch in oauth callback")
			return
		}

		code := q.Get("code")
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Gotack - Thiếu mã xác thực</title><style>body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#f8fafc}.card{background:#1e293b;padding:2.5rem;border-radius:1rem;text-align:center;box-shadow:0 10px 25px rgba(0,0,0,0.5);max-width:400px}h1{color:#ef4444;font-size:1.5rem;margin-bottom:0.75rem}p{color:#94a3b8;font-size:0.95rem;line-height:1.5}</style></head><body><div class="card"><h1>Lỗi xác thực</h1><p>Không tìm thấy authorization code.</p></div></body></html>`))
			errChan <- errors.New("missing authorization code in oauth callback")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Gotack - Đăng nhập ChatGPT thành công</title><style>body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#f8fafc}.card{background:#1e293b;padding:2.5rem;border-radius:1rem;text-align:center;box-shadow:0 10px 25px rgba(0,0,0,0.5);max-width:420px}h1{color:#10b981;font-size:1.5rem;margin-bottom:0.75rem}.icon{font-size:2.5rem;margin-bottom:1rem}p{color:#94a3b8;font-size:0.95rem;line-height:1.6}strong{color:#38bdf8}</style></head><body><div class="card"><div class="icon">✨</div><h1>Đăng nhập thành công!</h1><p>Tài khoản ChatGPT đã được liên kết với <strong>Gotack</strong>.<br>Bạn có thể đóng tab trình duyệt này và quay lại ứng dụng.</p></div></body></html>`))
		codeChan <- code
	})

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if opts.OpenBrowser != nil {
		if err := opts.OpenBrowser(authURL); err != nil {
			return nil, fmt.Errorf("open browser for OAuth login: %w", err)
		}
	}

	loginCtx, cancelLogin := context.WithTimeout(ctx, opts.LoginTimeout)
	defer cancelLogin()

	var authCode string
	select {
	case <-loginCtx.Done():
		return nil, fmt.Errorf("login timed out waiting for browser callback: %w", loginCtx.Err())
	case err := <-errChan:
		return nil, err
	case authCode = <-codeChan:
	}

	return ExchangeCode(ctx, opts, authCode, verifier, redirectURI)
}

func ExchangeCode(ctx context.Context, opts Options, code, verifier, redirectURI string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", opts.ClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tok Token
	if err := json.Unmarshal(bodyBytes, &tok); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if tok.AccessToken == "" {
		return nil, errors.New("token response missing access_token")
	}

	if tok.ExpiresIn > 0 {
		tok.ExpiresAt = time.Now().Unix() + int64(tok.ExpiresIn)
	}

	if tok.IDToken != "" {
		metadata := ParseIDTokenMetadata(tok.IDToken)
		tok.AccountEmail = metadata.Email
		tok.AccountPlan = metadata.Plan
		tok.AccountID = metadata.AccountID
		tok.ChatGPTUserID = metadata.ChatGPTUserID
		tok.AccountFedRAMP = metadata.AccountFedRAMP
	}

	return &tok, nil
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(s)
}
