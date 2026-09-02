package openaioauth

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

func TestGeneratePKCE(t *testing.T) {
	v, c, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}
	if len(v) == 0 {
		t.Fatal("GeneratePKCE() returned empty verifier")
	}
	if len(c) == 0 {
		t.Fatal("GeneratePKCE() returned empty challenge")
	}
	if v == c {
		t.Fatal("verifier and challenge should differ")
	}
}

func TestParseIDTokenClaims(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/profile":{"email":"user@example.com"},"https://api.openai.com/auth":{"chatgpt_plan_type":"plus","chatgpt_account_id":"acct_123","chatgpt_user_id":"user_123","chatgpt_account_is_fedramp":true}}`))
	jwt := fmt.Sprintf("%s.%s.signature", header, claims)

	email, plan := ParseIDTokenClaims(jwt)
	if email != "user@example.com" {
		t.Errorf("got email %q, want user@example.com", email)
	}
	if plan != "plus" {
		t.Errorf("got plan %q, want plus", plan)
	}
	metadata := ParseIDTokenMetadata(jwt)
	if metadata.AccountID != "acct_123" || metadata.ChatGPTUserID != "user_123" || !metadata.AccountFedRAMP {
		t.Fatalf("unexpected account metadata: %+v", metadata)
	}
}

func TestOAuthLogin(t *testing.T) {
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

	opts := DefaultOptions()
	opts.TokenURL = tokenServer.URL
	opts.Port = 14560 // Use a distinct port for testing
	opts.LoginTimeout = 5 * time.Second
	opts.OpenBrowser = func(authURL string) error {
		// Simulate browser redirect back to callback
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

	ctx := context.Background()
	token, err := StartLogin(ctx, opts)
	if err != nil {
		t.Fatalf("StartLogin() error = %v", err)
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
