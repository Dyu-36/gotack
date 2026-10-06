package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestWithOpenAIOAuthDefaults(t *testing.T) {
	customClient := &http.Client{Timeout: 11 * time.Second}
	customBrowser := func(string) error { return nil }
	tests := []struct {
		name         string
		input        OpenAIOAuthOptions
		wantClient   *http.Client
		wantClientID string
		wantAuthURL  string
		wantTokenURL string
		wantPort     int
		wantTimeout  time.Duration
	}{
		{
			name:         "fills missing values",
			input:        OpenAIOAuthOptions{},
			wantClientID: defaultOpenAIClientID,
			wantAuthURL:  defaultOpenAIAuthURL,
			wantTokenURL: defaultOpenAITokenURL,
			wantPort:     defaultOpenAIRedirectPort,
			wantTimeout:  3 * time.Minute,
		},
		{
			name: "preserves overrides",
			input: OpenAIOAuthOptions{
				ClientID:     "custom-client",
				AuthURL:      "https://auth.example.test",
				TokenURL:     "https://token.example.test",
				Port:         1234,
				HTTPClient:   customClient,
				OpenBrowser:  customBrowser,
				LoginTimeout: 11 * time.Second,
			},
			wantClient:   customClient,
			wantClientID: "custom-client",
			wantAuthURL:  "https://auth.example.test",
			wantTokenURL: "https://token.example.test",
			wantPort:     1234,
			wantTimeout:  11 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withOpenAIOAuthDefaults(tt.input)
			if got.ClientID != tt.wantClientID || got.AuthURL != tt.wantAuthURL || got.TokenURL != tt.wantTokenURL {
				t.Fatalf("oauth endpoints = (%q, %q, %q), want (%q, %q, %q)", got.ClientID, got.AuthURL, got.TokenURL, tt.wantClientID, tt.wantAuthURL, tt.wantTokenURL)
			}
			if got.Port != tt.wantPort || got.LoginTimeout != tt.wantTimeout {
				t.Fatalf("oauth timing = (port %d, timeout %s), want (port %d, timeout %s)", got.Port, got.LoginTimeout, tt.wantPort, tt.wantTimeout)
			}
			if tt.wantClient != nil {
				if got.HTTPClient != tt.wantClient {
					t.Fatal("oauth HTTP client override was not preserved")
				}
				if got.OpenBrowser == nil {
					t.Fatal("oauth browser callback override was not preserved")
				}
				return
			}
			if got.HTTPClient == nil || got.HTTPClient.Timeout != 30*time.Second {
				t.Fatalf("default HTTP client = %#v, want 30s timeout", got.HTTPClient)
			}
		})
	}
}

func TestGenerateOpenAIPKCE(t *testing.T) {
	v, c, err := generateOpenAIPKCE()
	if err != nil {
		t.Fatalf("generateOpenAIPKCE() error = %v", err)
	}
	if len(v) == 0 {
		t.Fatal("generateOpenAIPKCE() returned empty verifier")
	}
	if len(c) == 0 {
		t.Fatal("generateOpenAIPKCE() returned empty challenge")
	}
	if v == c {
		t.Fatal("verifier and challenge should differ")
	}
}

func TestParseOpenAIIDTokenMetadata(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/profile":{"email":"user@example.com"},"https://api.openai.com/auth":{"chatgpt_plan_type":"plus","chatgpt_account_id":"acct_123","chatgpt_user_id":"user_123","chatgpt_account_is_fedramp":true}}`))
	jwt := fmt.Sprintf("%s.%s.signature", header, claims)

	metadata := parseOpenAIIDTokenMetadata(jwt)
	if metadata.Email != "user@example.com" {
		t.Errorf("got email %q, want user@example.com", metadata.Email)
	}
	if metadata.Plan != "plus" {
		t.Errorf("got plan %q, want plus", metadata.Plan)
	}
	if metadata.AccountID != "acct_123" || metadata.ChatGPTUserID != "user_123" || !metadata.AccountFedRAMP {
		t.Fatalf("unexpected account metadata: %+v", metadata)
	}
}

func TestOpenAIOAuthLogin(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		grantType := r.Form.Get("grant_type")
		w.Header().Set("Content-Type", "application/json")

		if grantType == "authorization_code" {
			code := r.Form.Get("code")
			verifier := r.Form.Get("code_verifier")
			if code != "test-auth-code" || verifier == "" {
				http.Error(w, "invalid code or verifier", http.StatusBadRequest)
				return
			}
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"chatgpt_user@test.local","https://api.openai.com/auth":{"chatgpt_plan_type":"plus","chatgpt_account_id":"acct_test"}}`))
			idToken := fmt.Sprintf("%s.%s.sig", header, payload)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-token-123",
				"refresh_token": "refresh-token-abc",
				"id_token":      idToken,
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
			return
		}

		http.Error(w, "unsupported grant type", http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	opts := DefaultOpenAIOAuthOptions()
	opts.TokenURL = tokenServer.URL
	opts.Port = 14560
	opts.LoginTimeout = 5 * time.Second
	opts.OpenBrowser = func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		redirectURI := u.Query().Get("redirect_uri")
		state := u.Query().Get("state")

		go func() {
			time.Sleep(50 * time.Millisecond)
			cbURL := fmt.Sprintf("%s?code=test-auth-code&state=%s", redirectURI, state)
			resp, err := http.Get(cbURL)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}

	token, err := StartOpenAIOAuthLogin(context.Background(), opts)
	if err != nil {
		t.Fatalf("StartOpenAIOAuthLogin() error = %v", err)
	}
	if token.AccessToken != "access-token-123" {
		t.Errorf("got access token %q, want access-token-123", token.AccessToken)
	}
	if token.RefreshToken != "refresh-token-abc" {
		t.Errorf("got refresh token %q, want refresh-token-abc", token.RefreshToken)
	}
	if token.AccountEmail != "chatgpt_user@test.local" {
		t.Errorf("got email %q, want chatgpt_user@test.local", token.AccountEmail)
	}
	if token.AccountPlan != "plus" {
		t.Errorf("got plan %q, want plus", token.AccountPlan)
	}
	if token.AccountID != "acct_test" {
		t.Errorf("got account id %q, want acct_test", token.AccountID)
	}
}
