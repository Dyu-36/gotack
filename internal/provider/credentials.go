package provider

import (
	"os"
	"regexp"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

var simpleEnvCredentialRef = regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))$`)

func OAuthCredentialPresent(config crushapi.ProviderConfig) bool {
	raw := strings.TrimSpace(string(config.OAuth))
	return raw != "" && raw != "null" && raw != "{}"
}

func ResolvedCredential(config crushapi.ProviderConfig) (kind, value string, ok bool) {
	if OAuthCredentialPresent(config) {
		return "oauth", "", true
	}

	key := strings.TrimSpace(config.APIKey)
	if key == "" {
		return "", "", false
	}
	if match := simpleEnvCredentialRef.FindStringSubmatch(key); match != nil {
		name := match[1]
		if name == "" {
			name = match[2]
		}
		resolved, exists := os.LookupEnv(name)
		if !exists || strings.TrimSpace(resolved) == "" {
			return "", "", false
		}
		return "api_key", resolved, true
	}

	if strings.HasPrefix(key, "$") {
		return "", "", false
	}
	return "api_key", key, true
}
