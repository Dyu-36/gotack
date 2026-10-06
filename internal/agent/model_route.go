package agent

import (
	"net/http"
	"net/url"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/openai"
	azureSDK "github.com/openai/openai-go/v3/azure"
)

// anthropicBearerTransport prevents the SDK's environment credentials from
// leaking into Copilot requests while retaining its initiator transport.
type anthropicBearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t *anthropicBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	request := req.Clone(req.Context())
	request.Header.Del("X-Api-Key")
	request.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(request)
}

// buildRoutedAzureProvider selects Responses for catalog models regardless
// of whether the SDK's bundled model-name heuristic recognizes their IDs.
func (c *coordinator) buildRoutedAzureProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(endpoint.Hostname(), ".azure.com") && (endpoint.Path == "" || endpoint.Path == "/") {
		endpoint.Path = "/openai/v1"
	}
	return openai.New(
		openai.WithName(azure.Name),
		openai.WithBaseURL(endpoint.String()),
		openai.WithHeaders(headers),
		openai.WithSDKOptions(azureSDK.WithAPIKey(apiKey)),
		openai.WithUseResponsesAPI(),
		openai.WithResponsesAPIFunc(func(string) bool { return true }),
	)
}
