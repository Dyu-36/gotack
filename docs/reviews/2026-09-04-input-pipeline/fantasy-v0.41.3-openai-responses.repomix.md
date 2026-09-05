This file is a merged representation of a subset of the codebase, containing specifically included files, combined into a single document by Repomix.
The content has been processed where line numbers have been added, content has been formatted for parsing in markdown style.

# File Summary

## Purpose
This file contains a packed representation of a subset of the repository's contents that is considered the most important context.
It is designed to be easily consumable by AI systems for analysis, code review,
or other automated processes.

## File Format
The content is organized as follows:
1. This summary section
2. Repository information
3. Directory structure
4. Repository files (if enabled)
5. Multiple file entries, each consisting of:
  a. A header with the file path (## File: path/to/file)
  b. The full contents of the file in a code block

## Usage Guidelines
- This file should be treated as read-only. Any changes should be made to the
  original repository files, not this packed version.
- When processing this file, use the file path to distinguish
  between different files in the repository.
- Be aware that this file may contain sensitive information. Handle it with
  the same level of security as you would the original repository.
- Pay special attention to the Repository Description. These contain important context and guidelines specific to this project.

## Notes
- Some files may have been excluded based on .gitignore rules and Repomix's configuration
- Binary files are not included in this packed representation. Please refer to the Repository Structure section for a complete list of file paths, including binary files
- Only files matching these patterns are included: go.mod, providers/openai/responses_options.go, providers/openai/responses_language_model.go, providers/openai/language_model_hooks.go
- Line numbers have been added to the beginning of each line
- Content has been formatted for parsing in markdown style

# User Provided Header
Dependency review packet: Fantasy v0.41.3 OpenAI Responses implementation used by the audited Crush tree. Focus on PromptCacheKey serialization, cached-token accounting, encrypted reasoning capture, and whether stored reasoning is replayed. Repomix 1.13.1; 2026-09-04.

# Directory Structure
```
go.mod
providers/openai/language_model_hooks.go
providers/openai/responses_language_model.go
providers/openai/responses_options.go
```

# Files

## File: go.mod
```
  1: module charm.land/fantasy
  2: 
  3: go 1.26.6
  4: 
  5: require (
  6: 	charm.land/x/vcr v0.1.1
  7: 	cloud.google.com/go/auth v0.23.1
  8: 	github.com/anthropics/anthropic-sdk-go v1.63.1
  9: 	github.com/ardanlabs/kronk v1.31.5
 10: 	github.com/aws/aws-sdk-go-v2 v1.43.6
 11: 	github.com/aws/aws-sdk-go-v2/config v1.32.37
 12: 	github.com/aws/smithy-go v1.27.8
 13: 	github.com/charmbracelet/x/exp/slice v0.0.0-20250904123553-b4e2667e5ad5
 14: 	github.com/charmbracelet/x/exp/strings v0.1.0
 15: 	github.com/charmbracelet/x/json v0.2.0
 16: 	github.com/go-viper/mapstructure/v2 v2.5.0
 17: 	github.com/google/uuid v1.6.0
 18: 	github.com/joho/godotenv v1.5.1
 19: 	github.com/kaptinlin/jsonschema v0.9.8
 20: 	github.com/openai/openai-go/v3 v3.50.0
 21: 	github.com/stretchr/testify v1.12.0
 22: 	golang.org/x/net v0.58.0
 23: 	golang.org/x/oauth2 v0.36.0
 24: 	google.golang.org/genai v1.68.0
 25: )
 26: 
 27: require (
 28: 	cel.dev/expr v0.25.3 // indirect
 29: 	cloud.google.com/go v0.123.0 // indirect
 30: 	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
 31: 	cloud.google.com/go/compute/metadata v0.9.0 // indirect
 32: 	cloud.google.com/go/iam v1.13.0 // indirect
 33: 	cloud.google.com/go/monitoring v1.30.0 // indirect
 34: 	cloud.google.com/go/storage v1.64.0 // indirect
 35: 	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.22.0 // indirect
 36: 	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
 37: 	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.35.0 // indirect
 38: 	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.59.0 // indirect
 39: 	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.59.0 // indirect
 40: 	github.com/ardanlabs/jinja v1.6.0 // indirect
 41: 	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.18 // indirect
 42: 	github.com/aws/aws-sdk-go-v2/credentials v1.19.36 // indirect
 43: 	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.37 // indirect
 44: 	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.37 // indirect
 45: 	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.37 // indirect
 46: 	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.38 // indirect
 47: 	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.17 // indirect
 48: 	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.30 // indirect
 49: 	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.37 // indirect
 50: 	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.38 // indirect
 51: 	github.com/aws/aws-sdk-go-v2/service/s3 v1.107.2 // indirect
 52: 	github.com/aws/aws-sdk-go-v2/service/signin v1.5.6 // indirect
 53: 	github.com/aws/aws-sdk-go-v2/service/sso v1.33.6 // indirect
 54: 	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.6 // indirect
 55: 	github.com/aws/aws-sdk-go-v2/service/sts v1.45.6 // indirect
 56: 	github.com/bahlo/generic-list-go v0.2.0 // indirect
 57: 	github.com/beorn7/perks v1.0.1 // indirect
 58: 	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
 59: 	github.com/buger/jsonparser v1.1.2 // indirect
 60: 	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
 61: 	github.com/cespare/xxhash/v2 v2.3.0 // indirect
 62: 	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
 63: 	github.com/ebitengine/purego v0.10.2 // indirect
 64: 	github.com/envoyproxy/go-control-plane/envoy v1.39.0 // indirect
 65: 	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
 66: 	github.com/felixge/httpsnoop v1.1.0 // indirect
 67: 	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
 68: 	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
 69: 	github.com/go-logr/logr v1.4.4 // indirect
 70: 	github.com/go-logr/stdr v1.2.2 // indirect
 71: 	github.com/goccy/go-yaml v1.19.2 // indirect
 72: 	github.com/google/go-cmp v0.7.0 // indirect
 73: 	github.com/google/s2a-go v0.1.9 // indirect
 74: 	github.com/googleapis/enterprise-certificate-proxy v0.3.21 // indirect
 75: 	github.com/googleapis/gax-go/v2 v2.23.0 // indirect
 76: 	github.com/gorilla/websocket v1.5.3 // indirect
 77: 	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
 78: 	github.com/hashicorp/aws-sdk-go-base/v2 v2.0.0-beta.74 // indirect
 79: 	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
 80: 	github.com/hashicorp/go-getter v1.8.8 // indirect
 81: 	github.com/hashicorp/go-version v1.9.0 // indirect
 82: 	github.com/hybridgroup/yzma v1.23.0 // indirect
 83: 	github.com/invopop/jsonschema v0.14.0 // indirect
 84: 	github.com/jupiterrider/ffi v0.7.0 // indirect
 85: 	github.com/kaptinlin/jsonpointer v0.4.28 // indirect
 86: 	github.com/klauspost/compress v1.19.2 // indirect
 87: 	github.com/mitchellh/go-homedir v1.1.0 // indirect
 88: 	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
 89: 	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
 90: 	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
 91: 	github.com/prometheus/client_golang v1.24.1 // indirect
 92: 	github.com/prometheus/client_model v0.6.2 // indirect
 93: 	github.com/prometheus/common v0.70.1 // indirect
 94: 	github.com/prometheus/procfs v0.21.1 // indirect
 95: 	github.com/spiffe/go-spiffe/v2 v2.8.1 // indirect
 96: 	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
 97: 	github.com/tidwall/gjson v1.19.0 // indirect
 98: 	github.com/tidwall/match v1.1.1 // indirect
 99: 	github.com/tidwall/pretty v1.2.1 // indirect
100: 	github.com/tidwall/sjson v1.2.5 // indirect
101: 	github.com/ulikunitz/xz v0.5.16 // indirect
102: 	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
103: 	go.opentelemetry.io/contrib/detectors/gcp v1.45.0 // indirect
104: 	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.70.0 // indirect
105: 	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
106: 	go.opentelemetry.io/otel v1.45.0 // indirect
107: 	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.45.0 // indirect
108: 	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.45.0 // indirect
109: 	go.opentelemetry.io/otel/metric v1.45.0 // indirect
110: 	go.opentelemetry.io/otel/sdk v1.45.0 // indirect
111: 	go.opentelemetry.io/otel/sdk/metric v1.45.0 // indirect
112: 	go.opentelemetry.io/otel/trace v1.45.0 // indirect
113: 	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
114: 	go.yaml.in/yaml/v2 v2.4.4 // indirect
115: 	go.yaml.in/yaml/v4 v4.0.0-rc.3 // indirect
116: 	golang.org/x/crypto v0.55.0 // indirect
117: 	golang.org/x/image v0.45.0 // indirect
118: 	golang.org/x/sync v0.22.0 // indirect
119: 	golang.org/x/sys v0.47.0 // indirect
120: 	golang.org/x/text v0.41.0 // indirect
121: 	golang.org/x/time v0.15.0 // indirect
122: 	google.golang.org/api v0.293.0 // indirect
123: 	google.golang.org/genproto v0.0.0-20260818201246-1b0934165a6f // indirect
124: 	google.golang.org/genproto/googleapis/api v0.0.0-20260818201246-1b0934165a6f // indirect
125: 	google.golang.org/genproto/googleapis/rpc v0.0.0-20260818201246-1b0934165a6f // indirect
126: 	google.golang.org/grpc v1.83.0 // indirect
127: 	google.golang.org/protobuf v1.36.12 // indirect
128: 	gopkg.in/dnaeon/go-vcr.v4 v4.0.6-0.20251110073552-01de4eb40290 // indirect
129: 	gopkg.in/yaml.v3 v3.0.1 // indirect
130: )
```

## File: providers/openai/language_model_hooks.go
```go
  1: package openai
  2: 
  3: import (
  4: 	"encoding/base64"
  5: 	"fmt"
  6: 	"strings"
  7: 
  8: 	"charm.land/fantasy"
  9: 	"github.com/openai/openai-go/v3"
 10: 	"github.com/openai/openai-go/v3/packages/param"
 11: 	"github.com/openai/openai-go/v3/shared"
 12: )
 13: 
 14: // LanguageModelPrepareCallFunc is a function that prepares the call for the language model.
 15: type LanguageModelPrepareCallFunc = func(model fantasy.LanguageModel, params *openai.ChatCompletionNewParams, call fantasy.Call) ([]fantasy.CallWarning, error)
 16: 
 17: // LanguageModelMapFinishReasonFunc is a function that maps the finish reason for the language model.
 18: type LanguageModelMapFinishReasonFunc = func(finishReason string) fantasy.FinishReason
 19: 
 20: // LanguageModelUsageFunc is a function that calculates usage for the language model.
 21: type LanguageModelUsageFunc = func(choice openai.ChatCompletion) (fantasy.Usage, fantasy.ProviderOptionsData)
 22: 
 23: // LanguageModelExtraContentFunc is a function that adds extra content for the language model.
 24: type LanguageModelExtraContentFunc = func(choice openai.ChatCompletionChoice) []fantasy.Content
 25: 
 26: // LanguageModelStreamExtraFunc is a function that handles stream extra functionality for the language model.
 27: type LanguageModelStreamExtraFunc = func(chunk openai.ChatCompletionChunk, yield func(fantasy.StreamPart) bool, ctx map[string]any) (map[string]any, bool)
 28: 
 29: // LanguageModelStreamUsageFunc is a function that calculates stream usage for the language model.
 30: type LanguageModelStreamUsageFunc = func(chunk openai.ChatCompletionChunk, ctx map[string]any, metadata fantasy.ProviderMetadata) (fantasy.Usage, fantasy.ProviderMetadata)
 31: 
 32: // LanguageModelStreamProviderMetadataFunc is a function that handles stream provider metadata for the language model.
 33: type LanguageModelStreamProviderMetadataFunc = func(choice openai.ChatCompletionChoice, metadata fantasy.ProviderMetadata) fantasy.ProviderMetadata
 34: 
 35: // LanguageModelToPromptFunc is a function that handles converting fantasy prompts to openai sdk messages.
 36: type LanguageModelToPromptFunc = func(prompt fantasy.Prompt, provider, model string) ([]openai.ChatCompletionMessageParamUnion, []fantasy.CallWarning)
 37: 
 38: // DefaultPrepareCallFunc is the default implementation for preparing a call to the language model.
 39: func DefaultPrepareCallFunc(model fantasy.LanguageModel, params *openai.ChatCompletionNewParams, call fantasy.Call) ([]fantasy.CallWarning, error) {
 40: 	if call.ProviderOptions == nil {
 41: 		return nil, nil
 42: 	}
 43: 	var warnings []fantasy.CallWarning
 44: 	providerOptions := &ProviderOptions{}
 45: 	if v, ok := call.ProviderOptions[Name]; ok {
 46: 		providerOptions, ok = v.(*ProviderOptions)
 47: 		if !ok {
 48: 			return nil, &fantasy.Error{Title: "invalid argument", Message: "openai provider options should be *openai.ProviderOptions"}
 49: 		}
 50: 	}
 51: 
 52: 	if providerOptions.LogitBias != nil {
 53: 		params.LogitBias = providerOptions.LogitBias
 54: 	}
 55: 	if providerOptions.LogProbs != nil && providerOptions.TopLogProbs != nil {
 56: 		providerOptions.LogProbs = nil
 57: 	}
 58: 	if providerOptions.LogProbs != nil {
 59: 		params.Logprobs = param.NewOpt(*providerOptions.LogProbs)
 60: 	}
 61: 	if providerOptions.TopLogProbs != nil {
 62: 		params.TopLogprobs = param.NewOpt(*providerOptions.TopLogProbs)
 63: 	}
 64: 	if providerOptions.User != nil {
 65: 		params.User = param.NewOpt(*providerOptions.User)
 66: 	}
 67: 	if providerOptions.ParallelToolCalls != nil {
 68: 		params.ParallelToolCalls = param.NewOpt(*providerOptions.ParallelToolCalls)
 69: 	}
 70: 	if providerOptions.MaxCompletionTokens != nil {
 71: 		params.MaxCompletionTokens = param.NewOpt(*providerOptions.MaxCompletionTokens)
 72: 	}
 73: 
 74: 	if providerOptions.TextVerbosity != nil {
 75: 		params.Verbosity = openai.ChatCompletionNewParamsVerbosity(*providerOptions.TextVerbosity)
 76: 	}
 77: 	if providerOptions.Prediction != nil {
 78: 		// Convert map[string]any to ChatCompletionPredictionContentParam
 79: 		if content, ok := providerOptions.Prediction["content"]; ok {
 80: 			if contentStr, ok := content.(string); ok {
 81: 				params.Prediction = openai.ChatCompletionPredictionContentParam{
 82: 					Content: openai.ChatCompletionPredictionContentContentUnionParam{
 83: 						OfString: param.NewOpt(contentStr),
 84: 					},
 85: 				}
 86: 			}
 87: 		}
 88: 	}
 89: 	if providerOptions.Store != nil {
 90: 		params.Store = param.NewOpt(*providerOptions.Store)
 91: 	}
 92: 	if providerOptions.Metadata != nil {
 93: 		// Convert map[string]any to map[string]string
 94: 		metadata := make(map[string]string)
 95: 		for k, v := range providerOptions.Metadata {
 96: 			if str, ok := v.(string); ok {
 97: 				metadata[k] = str
 98: 			}
 99: 		}
100: 		params.Metadata = metadata
101: 	}
102: 	if providerOptions.PromptCacheKey != nil {
103: 		params.PromptCacheKey = param.NewOpt(*providerOptions.PromptCacheKey)
104: 	}
105: 	if providerOptions.SafetyIdentifier != nil {
106: 		params.SafetyIdentifier = param.NewOpt(*providerOptions.SafetyIdentifier)
107: 	}
108: 	if providerOptions.ServiceTier != nil {
109: 		params.ServiceTier = openai.ChatCompletionNewParamsServiceTier(*providerOptions.ServiceTier)
110: 	}
111: 
112: 	if providerOptions.ReasoningEffort != nil {
113: 		switch *providerOptions.ReasoningEffort {
114: 		case ReasoningEffortNone:
115: 			params.ReasoningEffort = shared.ReasoningEffortNone
116: 		case ReasoningEffortMinimal:
117: 			params.ReasoningEffort = shared.ReasoningEffortMinimal
118: 		case ReasoningEffortLow:
119: 			params.ReasoningEffort = shared.ReasoningEffortLow
120: 		case ReasoningEffortMedium:
121: 			params.ReasoningEffort = shared.ReasoningEffortMedium
122: 		case ReasoningEffortHigh:
123: 			params.ReasoningEffort = shared.ReasoningEffortHigh
124: 		case ReasoningEffortXHigh:
125: 			params.ReasoningEffort = shared.ReasoningEffortXhigh
126: 		case ReasoningEffortMax:
127: 			params.ReasoningEffort = shared.ReasoningEffortMax
128: 		default:
129: 			return nil, fmt.Errorf("reasoning model `%s` not supported", *providerOptions.ReasoningEffort)
130: 		}
131: 	}
132: 
133: 	if isReasoningModel(model.Model()) {
134: 		if providerOptions.LogitBias != nil {
135: 			params.LogitBias = nil
136: 			warnings = append(warnings, fantasy.CallWarning{
137: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
138: 				Setting: "LogitBias",
139: 				Message: "LogitBias is not supported for reasoning models",
140: 			})
141: 		}
142: 		if providerOptions.LogProbs != nil {
143: 			params.Logprobs = param.Opt[bool]{}
144: 			warnings = append(warnings, fantasy.CallWarning{
145: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
146: 				Setting: "Logprobs",
147: 				Message: "Logprobs is not supported for reasoning models",
148: 			})
149: 		}
150: 		if providerOptions.TopLogProbs != nil {
151: 			params.TopLogprobs = param.Opt[int64]{}
152: 			warnings = append(warnings, fantasy.CallWarning{
153: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
154: 				Setting: "TopLogprobs",
155: 				Message: "TopLogprobs is not supported for reasoning models",
156: 			})
157: 		}
158: 	}
159: 
160: 	// Handle service tier validation
161: 	if providerOptions.ServiceTier != nil {
162: 		serviceTier := *providerOptions.ServiceTier
163: 		if serviceTier == "flex" && !supportsFlexProcessing(model.Model()) {
164: 			params.ServiceTier = ""
165: 			warnings = append(warnings, fantasy.CallWarning{
166: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
167: 				Setting: "ServiceTier",
168: 				Details: "flex processing is only available for o3, o4-mini, and gpt-5 models",
169: 			})
170: 		} else if serviceTier == "priority" && !supportsPriorityProcessing(model.Model()) {
171: 			params.ServiceTier = ""
172: 			warnings = append(warnings, fantasy.CallWarning{
173: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
174: 				Setting: "ServiceTier",
175: 				Details: "priority processing is only available for supported models (gpt-4, gpt-5, gpt-5-mini, o3, o4-mini) and requires Enterprise access. gpt-5-nano is not supported",
176: 			})
177: 		}
178: 	}
179: 	return warnings, nil
180: }
181: 
182: // DefaultMapFinishReasonFunc is the default implementation for mapping finish reasons.
183: func DefaultMapFinishReasonFunc(finishReason string) fantasy.FinishReason {
184: 	switch finishReason {
185: 	case "stop":
186: 		return fantasy.FinishReasonStop
187: 	case "length":
188: 		return fantasy.FinishReasonLength
189: 	case "content_filter":
190: 		return fantasy.FinishReasonContentFilter
191: 	case "function_call", "tool_calls":
192: 		return fantasy.FinishReasonToolCalls
193: 	default:
194: 		return fantasy.FinishReasonUnknown
195: 	}
196: }
197: 
198: // DefaultUsageFunc is the default implementation for calculating usage.
199: func DefaultUsageFunc(response openai.ChatCompletion) (fantasy.Usage, fantasy.ProviderOptionsData) {
200: 	completionTokenDetails := response.Usage.CompletionTokensDetails
201: 	promptTokenDetails := response.Usage.PromptTokensDetails
202: 
203: 	// Build provider metadata
204: 	providerMetadata := &ProviderMetadata{}
205: 
206: 	// Add logprobs if available
207: 	if len(response.Choices) > 0 && len(response.Choices[0].Logprobs.Content) > 0 {
208: 		providerMetadata.Logprobs = response.Choices[0].Logprobs.Content
209: 	}
210: 
211: 	// Add prediction tokens if available
212: 	if completionTokenDetails.AcceptedPredictionTokens > 0 || completionTokenDetails.RejectedPredictionTokens > 0 {
213: 		if completionTokenDetails.AcceptedPredictionTokens > 0 {
214: 			providerMetadata.AcceptedPredictionTokens = completionTokenDetails.AcceptedPredictionTokens
215: 		}
216: 		if completionTokenDetails.RejectedPredictionTokens > 0 {
217: 			providerMetadata.RejectedPredictionTokens = completionTokenDetails.RejectedPredictionTokens
218: 		}
219: 	}
220: 	// OpenAI reports prompt_tokens INCLUDING cached tokens. Subtract to avoid double-counting.
221: 	inputTokens := max(response.Usage.PromptTokens-promptTokenDetails.CachedTokens, 0)
222: 	providerMetadata.ExtraFields = ExtractExtraFields(response.Usage.JSON.ExtraFields)
223: 	return fantasy.Usage{
224: 		InputTokens:     inputTokens,
225: 		OutputTokens:    response.Usage.CompletionTokens,
226: 		TotalTokens:     response.Usage.TotalTokens,
227: 		ReasoningTokens: completionTokenDetails.ReasoningTokens,
228: 		CacheReadTokens: promptTokenDetails.CachedTokens,
229: 	}, providerMetadata
230: }
231: 
232: // DefaultStreamUsageFunc is the default implementation for calculating stream usage.
233: func DefaultStreamUsageFunc(chunk openai.ChatCompletionChunk, _ map[string]any, metadata fantasy.ProviderMetadata) (fantasy.Usage, fantasy.ProviderMetadata) {
234: 	if chunk.Usage.TotalTokens == 0 {
235: 		return fantasy.Usage{}, nil
236: 	}
237: 	streamProviderMetadata := &ProviderMetadata{}
238: 	if metadata != nil {
239: 		if providerMetadata, ok := metadata[Name]; ok {
240: 			converted, ok := providerMetadata.(*ProviderMetadata)
241: 			if ok {
242: 				streamProviderMetadata = converted
243: 			}
244: 		}
245: 	}
246: 	// we do this here because the acc does not add prompt details
247: 	completionTokenDetails := chunk.Usage.CompletionTokensDetails
248: 	promptTokenDetails := chunk.Usage.PromptTokensDetails
249: 	// OpenAI reports prompt_tokens INCLUDING cached tokens. Subtract to avoid double-counting.
250: 	inputTokens := max(chunk.Usage.PromptTokens-promptTokenDetails.CachedTokens, 0)
251: 	usage := fantasy.Usage{
252: 		InputTokens:     inputTokens,
253: 		OutputTokens:    chunk.Usage.CompletionTokens,
254: 		TotalTokens:     chunk.Usage.TotalTokens,
255: 		ReasoningTokens: completionTokenDetails.ReasoningTokens,
256: 		CacheReadTokens: promptTokenDetails.CachedTokens,
257: 	}
258: 
259: 	// Add prediction tokens if available
260: 	if completionTokenDetails.AcceptedPredictionTokens > 0 || completionTokenDetails.RejectedPredictionTokens > 0 {
261: 		if completionTokenDetails.AcceptedPredictionTokens > 0 {
262: 			streamProviderMetadata.AcceptedPredictionTokens = completionTokenDetails.AcceptedPredictionTokens
263: 		}
264: 		if completionTokenDetails.RejectedPredictionTokens > 0 {
265: 			streamProviderMetadata.RejectedPredictionTokens = completionTokenDetails.RejectedPredictionTokens
266: 		}
267: 	}
268: 
269: 	streamProviderMetadata.ExtraFields = ExtractExtraFields(chunk.Usage.JSON.ExtraFields)
270: 
271: 	return usage, fantasy.ProviderMetadata{
272: 		Name: streamProviderMetadata,
273: 	}
274: }
275: 
276: // DefaultStreamProviderMetadataFunc is the default implementation for handling stream provider metadata.
277: func DefaultStreamProviderMetadataFunc(choice openai.ChatCompletionChoice, metadata fantasy.ProviderMetadata) fantasy.ProviderMetadata {
278: 	if metadata == nil {
279: 		metadata = fantasy.ProviderMetadata{}
280: 	}
281: 	streamProviderMetadata, ok := metadata[Name]
282: 	if !ok {
283: 		streamProviderMetadata = &ProviderMetadata{}
284: 	}
285: 	if converted, ok := streamProviderMetadata.(*ProviderMetadata); ok {
286: 		converted.Logprobs = choice.Logprobs.Content
287: 		metadata[Name] = converted
288: 	}
289: 	return metadata
290: }
291: 
292: // DefaultToPrompt converts a fantasy prompt to OpenAI format with default handling.
293: func DefaultToPrompt(prompt fantasy.Prompt, _, _ string) ([]openai.ChatCompletionMessageParamUnion, []fantasy.CallWarning) {
294: 	var messages []openai.ChatCompletionMessageParamUnion
295: 	var warnings []fantasy.CallWarning
296: 	for _, msg := range prompt {
297: 		switch msg.Role {
298: 		case fantasy.MessageRoleSystem:
299: 			var systemPromptParts []string
300: 			for _, c := range msg.Content {
301: 				if c.GetType() != fantasy.ContentTypeText {
302: 					warnings = append(warnings, fantasy.CallWarning{
303: 						Type:    fantasy.CallWarningTypeOther,
304: 						Message: "system prompt can only have text content",
305: 					})
306: 					continue
307: 				}
308: 				textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
309: 				if !ok {
310: 					warnings = append(warnings, fantasy.CallWarning{
311: 						Type:    fantasy.CallWarningTypeOther,
312: 						Message: "system prompt text part does not have the right type",
313: 					})
314: 					continue
315: 				}
316: 				text := textPart.Text
317: 				if strings.TrimSpace(text) != "" {
318: 					systemPromptParts = append(systemPromptParts, textPart.Text)
319: 				}
320: 			}
321: 			if len(systemPromptParts) == 0 {
322: 				warnings = append(warnings, fantasy.CallWarning{
323: 					Type:    fantasy.CallWarningTypeOther,
324: 					Message: "system prompt has no text parts",
325: 				})
326: 				continue
327: 			}
328: 			messages = append(messages, openai.SystemMessage(strings.Join(systemPromptParts, "\n")))
329: 		case fantasy.MessageRoleUser:
330: 			// simple user message just text content
331: 			if len(msg.Content) == 1 && msg.Content[0].GetType() == fantasy.ContentTypeText {
332: 				textPart, ok := fantasy.AsContentType[fantasy.TextPart](msg.Content[0])
333: 				if !ok {
334: 					warnings = append(warnings, fantasy.CallWarning{
335: 						Type:    fantasy.CallWarningTypeOther,
336: 						Message: "user message text part does not have the right type",
337: 					})
338: 					continue
339: 				}
340: 				messages = append(messages, openai.UserMessage(textPart.Text))
341: 				continue
342: 			}
343: 			// text content and attachments
344: 			// for now we only support image content later we need to check
345: 			// TODO: add the supported media types to the language model so we
346: 			//  can use that to validate the data here.
347: 			var content []openai.ChatCompletionContentPartUnionParam
348: 			for _, c := range msg.Content {
349: 				switch c.GetType() {
350: 				case fantasy.ContentTypeText:
351: 					textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
352: 					if !ok {
353: 						warnings = append(warnings, fantasy.CallWarning{
354: 							Type:    fantasy.CallWarningTypeOther,
355: 							Message: "user message text part does not have the right type",
356: 						})
357: 						continue
358: 					}
359: 					content = append(content, openai.ChatCompletionContentPartUnionParam{
360: 						OfText: &openai.ChatCompletionContentPartTextParam{
361: 							Text: textPart.Text,
362: 						},
363: 					})
364: 				case fantasy.ContentTypeFile:
365: 					filePart, ok := fantasy.AsContentType[fantasy.FilePart](c)
366: 					if !ok {
367: 						warnings = append(warnings, fantasy.CallWarning{
368: 							Type:    fantasy.CallWarningTypeOther,
369: 							Message: "user message file part does not have the right type",
370: 						})
371: 						continue
372: 					}
373: 
374: 					switch {
375: 					case strings.HasPrefix(filePart.MediaType, "text/"):
376: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
377: 						documentBlock := openai.ChatCompletionContentPartFileFileParam{
378: 							FileData: param.NewOpt(base64Encoded),
379: 						}
380: 						content = append(content, openai.FileContentPart(documentBlock))
381: 
382: 					case strings.HasPrefix(filePart.MediaType, "image/"):
383: 						// Handle image files
384: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
385: 						data := "data:" + filePart.MediaType + ";base64," + base64Encoded
386: 						imageURL := openai.ChatCompletionContentPartImageImageURLParam{URL: data}
387: 
388: 						// Check for provider-specific options like image detail
389: 						if providerOptions, ok := filePart.ProviderOptions[Name]; ok {
390: 							if detail, ok := providerOptions.(*ProviderFileOptions); ok {
391: 								imageURL.Detail = detail.ImageDetail
392: 							}
393: 						}
394: 
395: 						imageBlock := openai.ChatCompletionContentPartImageParam{ImageURL: imageURL}
396: 						content = append(content, openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock})
397: 
398: 					case filePart.MediaType == "audio/wav":
399: 						// Handle WAV audio files
400: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
401: 						audioBlock := openai.ChatCompletionContentPartInputAudioParam{
402: 							InputAudio: openai.ChatCompletionContentPartInputAudioInputAudioParam{
403: 								Data:   base64Encoded,
404: 								Format: "wav",
405: 							},
406: 						}
407: 						content = append(content, openai.ChatCompletionContentPartUnionParam{OfInputAudio: &audioBlock})
408: 
409: 					case filePart.MediaType == "audio/mpeg" || filePart.MediaType == "audio/mp3":
410: 						// Handle MP3 audio files
411: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
412: 						audioBlock := openai.ChatCompletionContentPartInputAudioParam{
413: 							InputAudio: openai.ChatCompletionContentPartInputAudioInputAudioParam{
414: 								Data:   base64Encoded,
415: 								Format: "mp3",
416: 							},
417: 						}
418: 						content = append(content, openai.ChatCompletionContentPartUnionParam{OfInputAudio: &audioBlock})
419: 
420: 					case filePart.MediaType == "application/pdf":
421: 						// Handle PDF files
422: 						dataStr := string(filePart.Data)
423: 
424: 						// Check if data looks like a file ID (starts with "file-")
425: 						if strings.HasPrefix(dataStr, "file-") {
426: 							fileBlock := openai.ChatCompletionContentPartFileParam{
427: 								File: openai.ChatCompletionContentPartFileFileParam{
428: 									FileID: param.NewOpt(dataStr),
429: 								},
430: 							}
431: 							content = append(content, openai.ChatCompletionContentPartUnionParam{OfFile: &fileBlock})
432: 						} else {
433: 							// Handle as base64 data
434: 							base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
435: 							data := "data:application/pdf;base64," + base64Encoded
436: 
437: 							filename := filePart.Filename
438: 							if filename == "" {
439: 								// Generate default filename based on content index
440: 								filename = fmt.Sprintf("part-%d.pdf", len(content))
441: 							}
442: 
443: 							fileBlock := openai.ChatCompletionContentPartFileParam{
444: 								File: openai.ChatCompletionContentPartFileFileParam{
445: 									Filename: param.NewOpt(filename),
446: 									FileData: param.NewOpt(data),
447: 								},
448: 							}
449: 							content = append(content, openai.ChatCompletionContentPartUnionParam{OfFile: &fileBlock})
450: 						}
451: 
452: 					default:
453: 						warnings = append(warnings, fantasy.CallWarning{
454: 							Type:    fantasy.CallWarningTypeOther,
455: 							Message: fmt.Sprintf("file part media type %s not supported", filePart.MediaType),
456: 						})
457: 					}
458: 				}
459: 			}
460: 			if !hasVisibleUserContent(content) {
461: 				warnings = append(warnings, fantasy.CallWarning{
462: 					Type:    fantasy.CallWarningTypeOther,
463: 					Message: "dropping empty user message (contains neither user-facing content nor tool results)",
464: 				})
465: 				continue
466: 			}
467: 			messages = append(messages, openai.UserMessage(content))
468: 		case fantasy.MessageRoleAssistant:
469: 			// simple assistant message just text content
470: 			if len(msg.Content) == 1 && msg.Content[0].GetType() == fantasy.ContentTypeText {
471: 				textPart, ok := fantasy.AsContentType[fantasy.TextPart](msg.Content[0])
472: 				if !ok {
473: 					warnings = append(warnings, fantasy.CallWarning{
474: 						Type:    fantasy.CallWarningTypeOther,
475: 						Message: "assistant message text part does not have the right type",
476: 					})
477: 					continue
478: 				}
479: 				messages = append(messages, openai.AssistantMessage(textPart.Text))
480: 				continue
481: 			}
482: 			assistantMsg := openai.ChatCompletionAssistantMessageParam{
483: 				Role: "assistant",
484: 			}
485: 			for _, c := range msg.Content {
486: 				switch c.GetType() {
487: 				case fantasy.ContentTypeText:
488: 					textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
489: 					if !ok {
490: 						warnings = append(warnings, fantasy.CallWarning{
491: 							Type:    fantasy.CallWarningTypeOther,
492: 							Message: "assistant message text part does not have the right type",
493: 						})
494: 						continue
495: 					}
496: 					assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
497: 						OfString: param.NewOpt(textPart.Text),
498: 					}
499: 				case fantasy.ContentTypeToolCall:
500: 					toolCallPart, ok := fantasy.AsContentType[fantasy.ToolCallPart](c)
501: 					if !ok {
502: 						warnings = append(warnings, fantasy.CallWarning{
503: 							Type:    fantasy.CallWarningTypeOther,
504: 							Message: "assistant message tool part does not have the right type",
505: 						})
506: 						continue
507: 					}
508: 					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls,
509: 						openai.ChatCompletionMessageToolCallUnionParam{
510: 							OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
511: 								ID:   toolCallPart.ToolCallID,
512: 								Type: "function",
513: 								Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
514: 									Name:      toolCallPart.ToolName,
515: 									Arguments: toolCallPart.Input,
516: 								},
517: 							},
518: 						})
519: 				}
520: 			}
521: 			if !hasVisibleAssistantContent(&assistantMsg) {
522: 				warnings = append(warnings, fantasy.CallWarning{
523: 					Type:    fantasy.CallWarningTypeOther,
524: 					Message: "dropping empty assistant message (contains neither user-facing content nor tool calls)",
525: 				})
526: 				continue
527: 			}
528: 			messages = append(messages, openai.ChatCompletionMessageParamUnion{
529: 				OfAssistant: &assistantMsg,
530: 			})
531: 		case fantasy.MessageRoleTool:
532: 			for _, c := range msg.Content {
533: 				if c.GetType() != fantasy.ContentTypeToolResult {
534: 					warnings = append(warnings, fantasy.CallWarning{
535: 						Type:    fantasy.CallWarningTypeOther,
536: 						Message: "tool message can only have tool result content",
537: 					})
538: 					continue
539: 				}
540: 
541: 				toolResultPart, ok := fantasy.AsContentType[fantasy.ToolResultPart](c)
542: 				if !ok {
543: 					warnings = append(warnings, fantasy.CallWarning{
544: 						Type:    fantasy.CallWarningTypeOther,
545: 						Message: "tool message result part does not have the right type",
546: 					})
547: 					continue
548: 				}
549: 
550: 				switch toolResultPart.Output.GetType() {
551: 				case fantasy.ToolResultContentTypeText:
552: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](toolResultPart.Output)
553: 					if !ok {
554: 						warnings = append(warnings, fantasy.CallWarning{
555: 							Type:    fantasy.CallWarningTypeOther,
556: 							Message: "tool result output does not have the right type",
557: 						})
558: 						continue
559: 					}
560: 					messages = append(messages, openai.ToolMessage(output.Text, toolResultPart.ToolCallID))
561: 				case fantasy.ToolResultContentTypeError:
562: 					// TODO: check if better handling is needed
563: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](toolResultPart.Output)
564: 					if !ok {
565: 						warnings = append(warnings, fantasy.CallWarning{
566: 							Type:    fantasy.CallWarningTypeOther,
567: 							Message: "tool result output does not have the right type",
568: 						})
569: 						continue
570: 					}
571: 					messages = append(messages, openai.ToolMessage(output.Error.Error(), toolResultPart.ToolCallID))
572: 				case fantasy.ToolResultContentTypeMedia:
573: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](toolResultPart.Output)
574: 					if !ok {
575: 						warnings = append(warnings, fantasy.CallWarning{
576: 							Type:    fantasy.CallWarningTypeOther,
577: 							Message: "tool result output does not have the right type",
578: 						})
579: 						continue
580: 					}
581: 					// OpenAI Chat Completions tool messages cannot carry image
582: 					// or audio content directly; see ToolResultMediaMessages.
583: 					mediaMessages, mediaWarnings := ToolResultMediaMessages(output, toolResultPart.ToolCallID)
584: 					messages = append(messages, mediaMessages...)
585: 					warnings = append(warnings, mediaWarnings...)
586: 				default:
587: 					warnings = append(warnings, fantasy.CallWarning{
588: 						Type:    fantasy.CallWarningTypeOther,
589: 						Message: fmt.Sprintf("tool result output type %q not supported", toolResultPart.Output.GetType()),
590: 					})
591: 				}
592: 			}
593: 		}
594: 	}
595: 	return messages, warnings
596: }
597: 
598: // ToolResultMediaMessages maps a tool-result media output to the chat
599: // completions messages that convey it. OpenAI tool messages can only carry
600: // text, so this emits a text tool message (using any accompanying text, or a
601: // placeholder describing the media) to keep the tool_call/tool_result pairing
602: // valid, followed by a synthetic user message holding the actual image or
603: // audio content part so vision- and audio-capable models can see it.
604: //
605: // Unsupported media types produce only the text tool message plus a warning.
606: // This is shared with OpenAI-compatible providers, which face the same
607: // constraint.
608: func ToolResultMediaMessages(output fantasy.ToolResultOutputContentMedia, toolCallID string) ([]openai.ChatCompletionMessageParamUnion, []fantasy.CallWarning) {
609: 	placeholder := output.Text
610: 	if placeholder == "" {
611: 		placeholder = fmt.Sprintf("The tool returned %s content; see the following user message.", output.MediaType)
612: 	}
613: 	messages := []openai.ChatCompletionMessageParamUnion{openai.ToolMessage(placeholder, toolCallID)}
614: 
615: 	mediaPart, warning, emit := toolResultMediaUserPart(output)
616: 	if warning != nil {
617: 		return messages, []fantasy.CallWarning{*warning}
618: 	}
619: 	if emit {
620: 		messages = append(messages, openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{mediaPart}))
621: 	}
622: 	return messages, nil
623: }
624: 
625: // toolResultMediaUserPart maps a tool-result media output to an OpenAI chat
626: // completions user content part. It returns the content part, an optional
627: // warning, and whether the caller should emit the returned part.
628: func toolResultMediaUserPart(output fantasy.ToolResultOutputContentMedia) (openai.ChatCompletionContentPartUnionParam, *fantasy.CallWarning, bool) {
629: 	switch {
630: 	case strings.HasPrefix(output.MediaType, "image/"):
631: 		data := "data:" + output.MediaType + ";base64," + output.Data
632: 		imageBlock := openai.ChatCompletionContentPartImageParam{
633: 			ImageURL: openai.ChatCompletionContentPartImageImageURLParam{URL: data},
634: 		}
635: 		return openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock}, nil, true
636: 	case output.MediaType == "audio/wav", output.MediaType == "audio/mpeg", output.MediaType == "audio/mp3":
637: 		format := "wav"
638: 		if output.MediaType != "audio/wav" {
639: 			format = "mp3"
640: 		}
641: 		audioBlock := openai.ChatCompletionContentPartInputAudioParam{
642: 			InputAudio: openai.ChatCompletionContentPartInputAudioInputAudioParam{
643: 				Data:   output.Data,
644: 				Format: format,
645: 			},
646: 		}
647: 		return openai.ChatCompletionContentPartUnionParam{OfInputAudio: &audioBlock}, nil, true
648: 	default:
649: 		return openai.ChatCompletionContentPartUnionParam{}, &fantasy.CallWarning{
650: 			Type:    fantasy.CallWarningTypeOther,
651: 			Message: fmt.Sprintf("tool result media type %s not supported, sending text placeholder only", output.MediaType),
652: 		}, false
653: 	}
654: }
655: 
656: func hasVisibleUserContent(content []openai.ChatCompletionContentPartUnionParam) bool {
657: 	for _, part := range content {
658: 		if part.OfText != nil || part.OfImageURL != nil || part.OfInputAudio != nil || part.OfFile != nil {
659: 			return true
660: 		}
661: 	}
662: 	return false
663: }
664: 
665: func hasVisibleAssistantContent(msg *openai.ChatCompletionAssistantMessageParam) bool {
666: 	// Check if there's text content
667: 	if !param.IsOmitted(msg.Content.OfString) || len(msg.Content.OfArrayOfContentParts) > 0 {
668: 		return true
669: 	}
670: 	// Check if there are tool calls
671: 	if len(msg.ToolCalls) > 0 {
672: 		return true
673: 	}
674: 	return false
675: }
```

## File: providers/openai/responses_language_model.go
```go
   1: package openai
   2: 
   3: import (
   4: 	"context"
   5: 	"encoding/base64"
   6: 	"errors"
   7: 	"fmt"
   8: 	"io"
   9: 	"reflect"
  10: 	"strings"
  11: 
  12: 	"charm.land/fantasy"
  13: 	"charm.land/fantasy/object"
  14: 	"charm.land/fantasy/schema"
  15: 	"github.com/google/uuid"
  16: 	"github.com/openai/openai-go/v3"
  17: 	"github.com/openai/openai-go/v3/packages/param"
  18: 	"github.com/openai/openai-go/v3/responses"
  19: 	"github.com/openai/openai-go/v3/shared"
  20: )
  21: 
  22: const topLogprobsMax = 20
  23: 
  24: type responsesLanguageModel struct {
  25: 	provider   string
  26: 	modelID    string
  27: 	client     openai.Client
  28: 	objectMode fantasy.ObjectMode
  29: }
  30: 
  31: // newResponsesLanguageModel implements a responses api model.
  32: func newResponsesLanguageModel(modelID string, provider string, client openai.Client, objectMode fantasy.ObjectMode) responsesLanguageModel {
  33: 	return responsesLanguageModel{
  34: 		modelID:    modelID,
  35: 		provider:   provider,
  36: 		client:     client,
  37: 		objectMode: objectMode,
  38: 	}
  39: }
  40: 
  41: func (o responsesLanguageModel) Model() string {
  42: 	return o.modelID
  43: }
  44: 
  45: func (o responsesLanguageModel) Provider() string {
  46: 	return o.provider
  47: }
  48: 
  49: type responsesModelConfig struct {
  50: 	isReasoningModel           bool
  51: 	systemMessageMode          string
  52: 	requiredAutoTruncation     bool
  53: 	supportsFlexProcessing     bool
  54: 	supportsPriorityProcessing bool
  55: }
  56: 
  57: func getResponsesModelConfig(modelID string) responsesModelConfig {
  58: 	supportsFlexProcessing := strings.HasPrefix(modelID, "o3") ||
  59: 		strings.Contains(modelID, "-o3") || strings.Contains(modelID, "o4-mini") ||
  60: 		(strings.Contains(modelID, "gpt-5") && !strings.Contains(modelID, "gpt-5-chat"))
  61: 
  62: 	supportsPriorityProcessing := strings.Contains(strings.ToLower(modelID), "gpt-4") ||
  63: 		strings.Contains(strings.ToLower(modelID), "gpt-5-mini") ||
  64: 		(strings.Contains(strings.ToLower(modelID), "gpt-5") &&
  65: 			!strings.Contains(strings.ToLower(modelID), "gpt-5-nano") &&
  66: 			!strings.Contains(strings.ToLower(modelID), "gpt-5-chat")) ||
  67: 		strings.HasPrefix(modelID, "o3") ||
  68: 		strings.Contains(modelID, "-o3") ||
  69: 		strings.Contains(modelID, "o4-mini")
  70: 
  71: 	defaults := responsesModelConfig{
  72: 		requiredAutoTruncation:     false,
  73: 		systemMessageMode:          "system",
  74: 		supportsFlexProcessing:     supportsFlexProcessing,
  75: 		supportsPriorityProcessing: supportsPriorityProcessing,
  76: 	}
  77: 
  78: 	if strings.Contains(strings.ToLower(modelID), "gpt-5-chat") {
  79: 		return responsesModelConfig{
  80: 			isReasoningModel:           false,
  81: 			systemMessageMode:          defaults.systemMessageMode,
  82: 			requiredAutoTruncation:     defaults.requiredAutoTruncation,
  83: 			supportsFlexProcessing:     defaults.supportsFlexProcessing,
  84: 			supportsPriorityProcessing: defaults.supportsPriorityProcessing,
  85: 		}
  86: 	}
  87: 
  88: 	if strings.HasPrefix(modelID, "o1") || strings.Contains(modelID, "-o1") ||
  89: 		strings.HasPrefix(modelID, "o3") || strings.Contains(modelID, "-o3") ||
  90: 		strings.HasPrefix(modelID, "o4") || strings.Contains(modelID, "-o4") ||
  91: 		strings.HasPrefix(modelID, "oss") || strings.Contains(modelID, "-oss") ||
  92: 		strings.Contains(strings.ToLower(modelID), "gpt-5") ||
  93: 		strings.Contains(modelID, "codex-") || strings.Contains(modelID, "computer-use") {
  94: 		if strings.Contains(modelID, "o1-mini") || strings.Contains(modelID, "o1-preview") {
  95: 			return responsesModelConfig{
  96: 				isReasoningModel:           true,
  97: 				systemMessageMode:          "remove",
  98: 				requiredAutoTruncation:     defaults.requiredAutoTruncation,
  99: 				supportsFlexProcessing:     defaults.supportsFlexProcessing,
 100: 				supportsPriorityProcessing: defaults.supportsPriorityProcessing,
 101: 			}
 102: 		}
 103: 
 104: 		return responsesModelConfig{
 105: 			isReasoningModel:           true,
 106: 			systemMessageMode:          "developer",
 107: 			requiredAutoTruncation:     defaults.requiredAutoTruncation,
 108: 			supportsFlexProcessing:     defaults.supportsFlexProcessing,
 109: 			supportsPriorityProcessing: defaults.supportsPriorityProcessing,
 110: 		}
 111: 	}
 112: 
 113: 	return responsesModelConfig{
 114: 		isReasoningModel:           false,
 115: 		systemMessageMode:          defaults.systemMessageMode,
 116: 		requiredAutoTruncation:     defaults.requiredAutoTruncation,
 117: 		supportsFlexProcessing:     defaults.supportsFlexProcessing,
 118: 		supportsPriorityProcessing: defaults.supportsPriorityProcessing,
 119: 	}
 120: }
 121: 
 122: const (
 123: 	previousResponseIDHistoryError = "cannot combine previous_response_id with replayed conversation history; use either previous_response_id (server-side chaining) or explicit message replay, not both"
 124: 	previousResponseIDStoreError   = "previous_response_id requires store to be true; the current response will not be stored and cannot be used for further chaining"
 125: )
 126: 
 127: func (o responsesLanguageModel) prepareParams(call fantasy.Call) (*responses.ResponseNewParams, []fantasy.CallWarning, error) {
 128: 	var warnings []fantasy.CallWarning
 129: 	params := &responses.ResponseNewParams{}
 130: 
 131: 	modelConfig := getResponsesModelConfig(o.modelID)
 132: 
 133: 	if call.TopK != nil {
 134: 		warnings = append(warnings, fantasy.CallWarning{
 135: 			Type:    fantasy.CallWarningTypeUnsupportedSetting,
 136: 			Setting: "topK",
 137: 		})
 138: 	}
 139: 
 140: 	if call.PresencePenalty != nil {
 141: 		warnings = append(warnings, fantasy.CallWarning{
 142: 			Type:    fantasy.CallWarningTypeUnsupportedSetting,
 143: 			Setting: "presencePenalty",
 144: 		})
 145: 	}
 146: 
 147: 	if call.FrequencyPenalty != nil {
 148: 		warnings = append(warnings, fantasy.CallWarning{
 149: 			Type:    fantasy.CallWarningTypeUnsupportedSetting,
 150: 			Setting: "frequencyPenalty",
 151: 		})
 152: 	}
 153: 
 154: 	var openaiOptions *ResponsesProviderOptions
 155: 	if opts, ok := call.ProviderOptions[Name]; ok {
 156: 		if typedOpts, ok := opts.(*ResponsesProviderOptions); ok {
 157: 			openaiOptions = typedOpts
 158: 		}
 159: 	}
 160: 
 161: 	if openaiOptions != nil && openaiOptions.Store != nil {
 162: 		params.Store = param.NewOpt(*openaiOptions.Store)
 163: 	} else {
 164: 		params.Store = param.NewOpt(false)
 165: 	}
 166: 
 167: 	if openaiOptions != nil && openaiOptions.PreviousResponseID != nil && *openaiOptions.PreviousResponseID != "" {
 168: 		if err := validatePreviousResponseIDPrompt(call.Prompt); err != nil {
 169: 			return nil, warnings, err
 170: 		}
 171: 		if openaiOptions.Store == nil || !*openaiOptions.Store {
 172: 			return nil, warnings, errors.New(previousResponseIDStoreError)
 173: 		}
 174: 		params.PreviousResponseID = param.NewOpt(*openaiOptions.PreviousResponseID)
 175: 	}
 176: 
 177: 	storeEnabled := openaiOptions != nil && openaiOptions.Store != nil && *openaiOptions.Store
 178: 	input, inputWarnings := toResponsesPrompt(call.Prompt, modelConfig.systemMessageMode, storeEnabled)
 179: 	warnings = append(warnings, inputWarnings...)
 180: 
 181: 	var include []IncludeType
 182: 
 183: 	addInclude := func(key IncludeType) {
 184: 		include = append(include, key)
 185: 	}
 186: 
 187: 	topLogprobs := 0
 188: 	if openaiOptions != nil && openaiOptions.Logprobs != nil {
 189: 		switch v := openaiOptions.Logprobs.(type) {
 190: 		case bool:
 191: 			if v {
 192: 				topLogprobs = topLogprobsMax
 193: 			}
 194: 		case float64:
 195: 			topLogprobs = int(v)
 196: 		case int:
 197: 			topLogprobs = v
 198: 		}
 199: 	}
 200: 
 201: 	if topLogprobs > 0 {
 202: 		addInclude(IncludeMessageOutputTextLogprobs)
 203: 	}
 204: 
 205: 	params.Model = o.modelID
 206: 	params.Input = responses.ResponseNewParamsInputUnion{
 207: 		OfInputItemList: input,
 208: 	}
 209: 
 210: 	if call.Temperature != nil {
 211: 		params.Temperature = param.NewOpt(*call.Temperature)
 212: 	}
 213: 	if call.TopP != nil {
 214: 		params.TopP = param.NewOpt(*call.TopP)
 215: 	}
 216: 	if call.MaxOutputTokens != nil {
 217: 		params.MaxOutputTokens = param.NewOpt(*call.MaxOutputTokens)
 218: 	}
 219: 
 220: 	if openaiOptions != nil {
 221: 		if openaiOptions.MaxToolCalls != nil {
 222: 			params.MaxToolCalls = param.NewOpt(*openaiOptions.MaxToolCalls)
 223: 		}
 224: 		if openaiOptions.Metadata != nil {
 225: 			metadata := make(shared.Metadata)
 226: 			for k, v := range openaiOptions.Metadata {
 227: 				if str, ok := v.(string); ok {
 228: 					metadata[k] = str
 229: 				}
 230: 			}
 231: 			params.Metadata = metadata
 232: 		}
 233: 		if openaiOptions.ParallelToolCalls != nil {
 234: 			params.ParallelToolCalls = param.NewOpt(*openaiOptions.ParallelToolCalls)
 235: 		}
 236: 		if openaiOptions.User != nil {
 237: 			params.User = param.NewOpt(*openaiOptions.User)
 238: 		}
 239: 		if openaiOptions.Instructions != nil {
 240: 			params.Instructions = param.NewOpt(*openaiOptions.Instructions)
 241: 		}
 242: 		if openaiOptions.ServiceTier != nil {
 243: 			params.ServiceTier = responses.ResponseNewParamsServiceTier(*openaiOptions.ServiceTier)
 244: 		}
 245: 		if openaiOptions.PromptCacheKey != nil {
 246: 			params.PromptCacheKey = param.NewOpt(*openaiOptions.PromptCacheKey)
 247: 		}
 248: 		if openaiOptions.SafetyIdentifier != nil {
 249: 			params.SafetyIdentifier = param.NewOpt(*openaiOptions.SafetyIdentifier)
 250: 		}
 251: 		if topLogprobs > 0 {
 252: 			params.TopLogprobs = param.NewOpt(int64(topLogprobs))
 253: 		}
 254: 
 255: 		if len(openaiOptions.Include) > 0 {
 256: 			include = append(include, openaiOptions.Include...)
 257: 		}
 258: 
 259: 		if modelConfig.isReasoningModel && (openaiOptions.ReasoningEffort != nil || openaiOptions.ReasoningSummary != nil) {
 260: 			reasoning := shared.ReasoningParam{}
 261: 			if openaiOptions.ReasoningEffort != nil {
 262: 				reasoning.Effort = shared.ReasoningEffort(*openaiOptions.ReasoningEffort)
 263: 			}
 264: 			if openaiOptions.ReasoningSummary != nil {
 265: 				reasoning.Summary = shared.ReasoningSummary(*openaiOptions.ReasoningSummary)
 266: 			}
 267: 			params.Reasoning = reasoning
 268: 		}
 269: 	}
 270: 
 271: 	if modelConfig.requiredAutoTruncation {
 272: 		params.Truncation = responses.ResponseNewParamsTruncationAuto
 273: 	}
 274: 
 275: 	if len(include) > 0 {
 276: 		includeParams := make([]responses.ResponseIncludable, len(include))
 277: 		for i, inc := range include {
 278: 			includeParams[i] = responses.ResponseIncludable(string(inc))
 279: 		}
 280: 		params.Include = includeParams
 281: 	}
 282: 
 283: 	if modelConfig.isReasoningModel {
 284: 		if call.Temperature != nil {
 285: 			params.Temperature = param.Opt[float64]{}
 286: 			warnings = append(warnings, fantasy.CallWarning{
 287: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
 288: 				Setting: "temperature",
 289: 				Details: "temperature is not supported for reasoning models",
 290: 			})
 291: 		}
 292: 
 293: 		if call.TopP != nil {
 294: 			params.TopP = param.Opt[float64]{}
 295: 			warnings = append(warnings, fantasy.CallWarning{
 296: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
 297: 				Setting: "topP",
 298: 				Details: "topP is not supported for reasoning models",
 299: 			})
 300: 		}
 301: 	} else {
 302: 		if openaiOptions != nil {
 303: 			if openaiOptions.ReasoningEffort != nil {
 304: 				warnings = append(warnings, fantasy.CallWarning{
 305: 					Type:    fantasy.CallWarningTypeUnsupportedSetting,
 306: 					Setting: "reasoningEffort",
 307: 					Details: "reasoningEffort is not supported for non-reasoning models",
 308: 				})
 309: 			}
 310: 
 311: 			if openaiOptions.ReasoningSummary != nil {
 312: 				warnings = append(warnings, fantasy.CallWarning{
 313: 					Type:    fantasy.CallWarningTypeUnsupportedSetting,
 314: 					Setting: "reasoningSummary",
 315: 					Details: "reasoningSummary is not supported for non-reasoning models",
 316: 				})
 317: 			}
 318: 		}
 319: 	}
 320: 
 321: 	if openaiOptions != nil && openaiOptions.ServiceTier != nil {
 322: 		if *openaiOptions.ServiceTier == ServiceTierFlex && !modelConfig.supportsFlexProcessing {
 323: 			warnings = append(warnings, fantasy.CallWarning{
 324: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
 325: 				Setting: "serviceTier",
 326: 				Details: "flex processing is only available for o3, o4-mini, and gpt-5 models",
 327: 			})
 328: 			params.ServiceTier = ""
 329: 		}
 330: 
 331: 		if *openaiOptions.ServiceTier == ServiceTierPriority && !modelConfig.supportsPriorityProcessing {
 332: 			warnings = append(warnings, fantasy.CallWarning{
 333: 				Type:    fantasy.CallWarningTypeUnsupportedSetting,
 334: 				Setting: "serviceTier",
 335: 				Details: "priority processing is only available for supported models (gpt-4, gpt-5, gpt-5-mini, o3, o4-mini) and requires Enterprise access. gpt-5-nano is not supported",
 336: 			})
 337: 			params.ServiceTier = ""
 338: 		}
 339: 	}
 340: 
 341: 	tools, toolChoice, toolWarnings := toResponsesTools(call.Tools, call.ToolChoice, openaiOptions)
 342: 	warnings = append(warnings, toolWarnings...)
 343: 
 344: 	if len(tools) > 0 {
 345: 		params.Tools = tools
 346: 		params.ToolChoice = toolChoice
 347: 	}
 348: 
 349: 	return params, warnings, nil
 350: }
 351: 
 352: func validatePreviousResponseIDPrompt(prompt fantasy.Prompt) error {
 353: 	for _, msg := range prompt {
 354: 		switch msg.Role {
 355: 		case fantasy.MessageRoleSystem, fantasy.MessageRoleUser:
 356: 			continue
 357: 		default:
 358: 			return errors.New(previousResponseIDHistoryError)
 359: 		}
 360: 	}
 361: 	return nil
 362: }
 363: 
 364: func responsesProviderMetadata(responseID string) fantasy.ProviderMetadata {
 365: 	if responseID == "" {
 366: 		return fantasy.ProviderMetadata{}
 367: 	}
 368: 
 369: 	return fantasy.ProviderMetadata{
 370: 		Name: &ResponsesProviderMetadata{
 371: 			ResponseID: responseID,
 372: 		},
 373: 	}
 374: }
 375: 
 376: func responsesUsage(resp responses.Response) fantasy.Usage {
 377: 	// OpenAI reports input_tokens INCLUDING cached tokens. Subtract to avoid double-counting.
 378: 	inputTokens := max(resp.Usage.InputTokens-resp.Usage.InputTokensDetails.CachedTokens, 0)
 379: 	usage := fantasy.Usage{
 380: 		InputTokens:  inputTokens,
 381: 		OutputTokens: resp.Usage.OutputTokens,
 382: 		TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
 383: 	}
 384: 	if resp.Usage.OutputTokensDetails.ReasoningTokens != 0 {
 385: 		usage.ReasoningTokens = resp.Usage.OutputTokensDetails.ReasoningTokens
 386: 	}
 387: 	if resp.Usage.InputTokensDetails.CachedTokens != 0 {
 388: 		usage.CacheReadTokens = resp.Usage.InputTokensDetails.CachedTokens
 389: 	}
 390: 	return usage
 391: }
 392: 
 393: func toResponsesPrompt(prompt fantasy.Prompt, systemMessageMode string, store bool) (responses.ResponseInputParam, []fantasy.CallWarning) {
 394: 	var input responses.ResponseInputParam
 395: 	var warnings []fantasy.CallWarning
 396: 
 397: 	for _, msg := range prompt {
 398: 		switch msg.Role {
 399: 		case fantasy.MessageRoleSystem:
 400: 			var systemText string
 401: 			for _, c := range msg.Content {
 402: 				if c.GetType() != fantasy.ContentTypeText {
 403: 					warnings = append(warnings, fantasy.CallWarning{
 404: 						Type:    fantasy.CallWarningTypeOther,
 405: 						Message: "system prompt can only have text content",
 406: 					})
 407: 					continue
 408: 				}
 409: 				textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
 410: 				if !ok {
 411: 					warnings = append(warnings, fantasy.CallWarning{
 412: 						Type:    fantasy.CallWarningTypeOther,
 413: 						Message: "system prompt text part does not have the right type",
 414: 					})
 415: 					continue
 416: 				}
 417: 				if strings.TrimSpace(textPart.Text) != "" {
 418: 					systemText += textPart.Text
 419: 				}
 420: 			}
 421: 
 422: 			if systemText == "" {
 423: 				warnings = append(warnings, fantasy.CallWarning{
 424: 					Type:    fantasy.CallWarningTypeOther,
 425: 					Message: "system prompt has no text parts",
 426: 				})
 427: 				continue
 428: 			}
 429: 
 430: 			switch systemMessageMode {
 431: 			case "system":
 432: 				input = append(input, responses.ResponseInputItemParamOfMessage(systemText, responses.EasyInputMessageRoleSystem))
 433: 			case "developer":
 434: 				input = append(input, responses.ResponseInputItemParamOfMessage(systemText, responses.EasyInputMessageRoleDeveloper))
 435: 			case "remove":
 436: 				warnings = append(warnings, fantasy.CallWarning{
 437: 					Type:    fantasy.CallWarningTypeOther,
 438: 					Message: "system messages are removed for this model",
 439: 				})
 440: 			}
 441: 
 442: 		case fantasy.MessageRoleUser:
 443: 			var contentParts responses.ResponseInputMessageContentListParam
 444: 			for i, c := range msg.Content {
 445: 				switch c.GetType() {
 446: 				case fantasy.ContentTypeText:
 447: 					textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
 448: 					if !ok {
 449: 						warnings = append(warnings, fantasy.CallWarning{
 450: 							Type:    fantasy.CallWarningTypeOther,
 451: 							Message: "user message text part does not have the right type",
 452: 						})
 453: 						continue
 454: 					}
 455: 					contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
 456: 						OfInputText: &responses.ResponseInputTextParam{
 457: 							Type: "input_text",
 458: 							Text: textPart.Text,
 459: 						},
 460: 					})
 461: 
 462: 				case fantasy.ContentTypeFile:
 463: 					filePart, ok := fantasy.AsContentType[fantasy.FilePart](c)
 464: 					if !ok {
 465: 						warnings = append(warnings, fantasy.CallWarning{
 466: 							Type:    fantasy.CallWarningTypeOther,
 467: 							Message: "user message file part does not have the right type",
 468: 						})
 469: 						continue
 470: 					}
 471: 
 472: 					if strings.HasPrefix(filePart.MediaType, "image/") {
 473: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
 474: 						imageURL := fmt.Sprintf("data:%s;base64,%s", filePart.MediaType, base64Encoded)
 475: 						contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
 476: 							OfInputImage: &responses.ResponseInputImageParam{
 477: 								Type:     "input_image",
 478: 								ImageURL: param.NewOpt(imageURL),
 479: 							},
 480: 						})
 481: 					} else if filePart.MediaType == "application/pdf" {
 482: 						base64Encoded := base64.StdEncoding.EncodeToString(filePart.Data)
 483: 						fileData := fmt.Sprintf("data:application/pdf;base64,%s", base64Encoded)
 484: 						filename := filePart.Filename
 485: 						if filename == "" {
 486: 							filename = fmt.Sprintf("part-%d.pdf", i)
 487: 						}
 488: 						contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
 489: 							OfInputFile: &responses.ResponseInputFileParam{
 490: 								Type:     "input_file",
 491: 								Filename: param.NewOpt(filename),
 492: 								FileData: param.NewOpt(fileData),
 493: 							},
 494: 						})
 495: 					} else {
 496: 						warnings = append(warnings, fantasy.CallWarning{
 497: 							Type:    fantasy.CallWarningTypeOther,
 498: 							Message: fmt.Sprintf("file part media type %s not supported", filePart.MediaType),
 499: 						})
 500: 					}
 501: 				}
 502: 			}
 503: 
 504: 			if !hasVisibleResponsesUserContent(contentParts) {
 505: 				warnings = append(warnings, fantasy.CallWarning{
 506: 					Type:    fantasy.CallWarningTypeOther,
 507: 					Message: "dropping empty user message (contains neither user-facing content nor tool results)",
 508: 				})
 509: 				continue
 510: 			}
 511: 
 512: 			input = append(input, responses.ResponseInputItemParamOfMessage(contentParts, responses.EasyInputMessageRoleUser))
 513: 
 514: 		case fantasy.MessageRoleAssistant:
 515: 			startIdx := len(input)
 516: 			for _, c := range msg.Content {
 517: 				switch c.GetType() {
 518: 				case fantasy.ContentTypeText:
 519: 					textPart, ok := fantasy.AsContentType[fantasy.TextPart](c)
 520: 					if !ok {
 521: 						warnings = append(warnings, fantasy.CallWarning{
 522: 							Type:    fantasy.CallWarningTypeOther,
 523: 							Message: "assistant message text part does not have the right type",
 524: 						})
 525: 						continue
 526: 					}
 527: 					input = append(input, responses.ResponseInputItemParamOfMessage(textPart.Text, responses.EasyInputMessageRoleAssistant))
 528: 
 529: 				case fantasy.ContentTypeToolCall:
 530: 					toolCallPart, ok := fantasy.AsContentType[fantasy.ToolCallPart](c)
 531: 					if !ok {
 532: 						warnings = append(warnings, fantasy.CallWarning{
 533: 							Type:    fantasy.CallWarningTypeOther,
 534: 							Message: "assistant message tool call part does not have the right type",
 535: 						})
 536: 						continue
 537: 					}
 538: 
 539: 					if toolCallPart.ProviderExecuted {
 540: 						if store {
 541: 							// Round-trip provider-executed tools via
 542: 							// item_reference, letting the API resolve
 543: 							// the stored output item by ID.
 544: 							input = append(input, responses.ResponseInputItemParamOfItemReference(toolCallPart.ToolCallID))
 545: 						}
 546: 						// When store is disabled, server-side items are
 547: 						// ephemeral and cannot be referenced. Skip the
 548: 						// tool call; results are already omitted for
 549: 						// provider-executed tools.
 550: 						continue
 551: 					}
 552: 
 553: 					input = append(input, responses.ResponseInputItemParamOfFunctionCall(toolCallPart.Input, toolCallPart.ToolCallID, toolCallPart.ToolName))
 554: 				case fantasy.ContentTypeSource:
 555: 					// Source citations from web search are not a
 556: 					// recognised Responses API input type; skip.
 557: 					continue
 558: 				case fantasy.ContentTypeReasoning:
 559: 					// Reasoning items are always skipped during replay.
 560: 					// When store is enabled, the API already has them
 561: 					// persisted server-side. When store is disabled, the
 562: 					// item IDs are ephemeral and referencing them causes
 563: 					// "Item not found" errors. In both cases, replaying
 564: 					// reasoning inline is not supported by the API.
 565: 					continue
 566: 				}
 567: 			}
 568: 
 569: 			if !hasVisibleResponsesAssistantContent(input, startIdx) {
 570: 				warnings = append(warnings, fantasy.CallWarning{
 571: 					Type:    fantasy.CallWarningTypeOther,
 572: 					Message: "dropping empty assistant message (contains neither user-facing content nor tool calls)",
 573: 				})
 574: 				// Remove any items that were added during this iteration
 575: 				input = input[:startIdx]
 576: 				continue
 577: 			}
 578: 
 579: 		case fantasy.MessageRoleTool:
 580: 			for _, c := range msg.Content {
 581: 				if c.GetType() != fantasy.ContentTypeToolResult {
 582: 					warnings = append(warnings, fantasy.CallWarning{
 583: 						Type:    fantasy.CallWarningTypeOther,
 584: 						Message: "tool message can only have tool result content",
 585: 					})
 586: 					continue
 587: 				}
 588: 
 589: 				toolResultPart, ok := fantasy.AsContentType[fantasy.ToolResultPart](c)
 590: 				if !ok {
 591: 					warnings = append(warnings, fantasy.CallWarning{
 592: 						Type:    fantasy.CallWarningTypeOther,
 593: 						Message: "tool message result part does not have the right type",
 594: 					})
 595: 					continue
 596: 				}
 597: 
 598: 				// Provider-executed tool results (e.g. web search)
 599: 				// are already round-tripped via the tool call; skip.
 600: 				if toolResultPart.ProviderExecuted {
 601: 					continue
 602: 				}
 603: 
 604: 				var outputStr string
 605: 				var followupParts responses.ResponseInputMessageContentListParam
 606: 
 607: 				switch toolResultPart.Output.GetType() {
 608: 				case fantasy.ToolResultContentTypeText:
 609: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](toolResultPart.Output)
 610: 					if !ok {
 611: 						warnings = append(warnings, fantasy.CallWarning{
 612: 							Type:    fantasy.CallWarningTypeOther,
 613: 							Message: "tool result output does not have the right type",
 614: 						})
 615: 						continue
 616: 					}
 617: 					outputStr = output.Text
 618: 				case fantasy.ToolResultContentTypeError:
 619: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](toolResultPart.Output)
 620: 					if !ok {
 621: 						warnings = append(warnings, fantasy.CallWarning{
 622: 							Type:    fantasy.CallWarningTypeOther,
 623: 							Message: "tool result output does not have the right type",
 624: 						})
 625: 						continue
 626: 					}
 627: 					outputStr = output.Error.Error()
 628: 				case fantasy.ToolResultContentTypeMedia:
 629: 					output, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](toolResultPart.Output)
 630: 					if !ok {
 631: 						warnings = append(warnings, fantasy.CallWarning{
 632: 							Type:    fantasy.CallWarningTypeOther,
 633: 							Message: "tool result output does not have the right type",
 634: 						})
 635: 						continue
 636: 					}
 637: 					// The Responses API function_call_output only accepts a
 638: 					// string. Emit a text placeholder (preserving any
 639: 					// accompanying text) so the tool_call/tool_result pairing
 640: 					// stays valid, then attach the media as a synthetic user
 641: 					// input_image so vision-capable models still receive it.
 642: 					outputStr = output.Text
 643: 					if outputStr == "" {
 644: 						outputStr = fmt.Sprintf("The tool returned %s content; see the following user message.", output.MediaType)
 645: 					}
 646: 					if strings.HasPrefix(output.MediaType, "image/") {
 647: 						imageURL := fmt.Sprintf("data:%s;base64,%s", output.MediaType, output.Data)
 648: 						followupParts = append(followupParts, responses.ResponseInputContentUnionParam{
 649: 							OfInputImage: &responses.ResponseInputImageParam{
 650: 								Type:     "input_image",
 651: 								ImageURL: param.NewOpt(imageURL),
 652: 							},
 653: 						})
 654: 					} else {
 655: 						warnings = append(warnings, fantasy.CallWarning{
 656: 							Type:    fantasy.CallWarningTypeOther,
 657: 							Message: fmt.Sprintf("tool result media type %s not supported, sending text placeholder only", output.MediaType),
 658: 						})
 659: 					}
 660: 				default:
 661: 					warnings = append(warnings, fantasy.CallWarning{
 662: 						Type:    fantasy.CallWarningTypeOther,
 663: 						Message: fmt.Sprintf("tool result output type %q not supported", toolResultPart.Output.GetType()),
 664: 					})
 665: 					continue
 666: 				}
 667: 
 668: 				input = append(input, responses.ResponseInputItemParamOfFunctionCallOutput(toolResultPart.ToolCallID, outputStr))
 669: 				if len(followupParts) > 0 {
 670: 					input = append(input, responses.ResponseInputItemParamOfMessage(followupParts, responses.EasyInputMessageRoleUser))
 671: 				}
 672: 			}
 673: 		}
 674: 	}
 675: 
 676: 	return input, warnings
 677: }
 678: 
 679: func hasVisibleResponsesUserContent(content responses.ResponseInputMessageContentListParam) bool {
 680: 	return len(content) > 0
 681: }
 682: 
 683: func hasVisibleResponsesAssistantContent(items []responses.ResponseInputItemUnionParam, startIdx int) bool {
 684: 	// Check if we added any assistant content parts from this message
 685: 	for i := startIdx; i < len(items); i++ {
 686: 		if items[i].OfMessage != nil || items[i].OfFunctionCall != nil || items[i].OfItemReference != nil {
 687: 			return true
 688: 		}
 689: 	}
 690: 	return false
 691: }
 692: 
 693: func toResponsesTools(tools []fantasy.Tool, toolChoice *fantasy.ToolChoice, options *ResponsesProviderOptions) ([]responses.ToolUnionParam, responses.ResponseNewParamsToolChoiceUnion, []fantasy.CallWarning) {
 694: 	warnings := make([]fantasy.CallWarning, 0)
 695: 	var openaiTools []responses.ToolUnionParam
 696: 
 697: 	if len(tools) == 0 {
 698: 		return nil, responses.ResponseNewParamsToolChoiceUnion{}, nil
 699: 	}
 700: 
 701: 	strictJSONSchema := false
 702: 	if options != nil && options.StrictJSONSchema != nil {
 703: 		strictJSONSchema = *options.StrictJSONSchema
 704: 	}
 705: 
 706: 	for _, tool := range tools {
 707: 		if tool.GetType() == fantasy.ToolTypeFunction {
 708: 			ft, ok := tool.(fantasy.FunctionTool)
 709: 			if !ok {
 710: 				continue
 711: 			}
 712: 			openaiTools = append(openaiTools, responses.ToolUnionParam{
 713: 				OfFunction: &responses.FunctionToolParam{
 714: 					Name:        ft.Name,
 715: 					Description: param.NewOpt(ft.Description),
 716: 					Parameters:  ft.InputSchema,
 717: 					Strict:      param.NewOpt(strictJSONSchema),
 718: 					Type:        "function",
 719: 				},
 720: 			})
 721: 			continue
 722: 		}
 723: 		if tool.GetType() == fantasy.ToolTypeProviderDefined {
 724: 			pt, ok := tool.(fantasy.ProviderDefinedTool)
 725: 			if !ok {
 726: 				continue
 727: 			}
 728: 			switch pt.ID {
 729: 			case "web_search":
 730: 				openaiTools = append(openaiTools, toWebSearchToolParam(pt))
 731: 				continue
 732: 			}
 733: 		}
 734: 
 735: 		warnings = append(warnings, fantasy.CallWarning{
 736: 			Type:    fantasy.CallWarningTypeUnsupportedTool,
 737: 			Tool:    tool,
 738: 			Message: "tool is not supported",
 739: 		})
 740: 	}
 741: 
 742: 	if toolChoice == nil {
 743: 		return openaiTools, responses.ResponseNewParamsToolChoiceUnion{}, warnings
 744: 	}
 745: 
 746: 	var openaiToolChoice responses.ResponseNewParamsToolChoiceUnion
 747: 
 748: 	switch *toolChoice {
 749: 	case fantasy.ToolChoiceAuto:
 750: 		openaiToolChoice = responses.ResponseNewParamsToolChoiceUnion{
 751: 			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsAuto),
 752: 		}
 753: 	case fantasy.ToolChoiceNone:
 754: 		openaiToolChoice = responses.ResponseNewParamsToolChoiceUnion{
 755: 			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsNone),
 756: 		}
 757: 	case fantasy.ToolChoiceRequired:
 758: 		openaiToolChoice = responses.ResponseNewParamsToolChoiceUnion{
 759: 			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsRequired),
 760: 		}
 761: 	default:
 762: 		openaiToolChoice = responses.ResponseNewParamsToolChoiceUnion{
 763: 			OfFunctionTool: &responses.ToolChoiceFunctionParam{
 764: 				Type: "function",
 765: 				Name: string(*toolChoice),
 766: 			},
 767: 		}
 768: 	}
 769: 
 770: 	return openaiTools, openaiToolChoice, warnings
 771: }
 772: 
 773: func (o responsesLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
 774: 	params, warnings, err := o.prepareParams(call)
 775: 	if err != nil {
 776: 		return nil, err
 777: 	}
 778: 
 779: 	response, err := o.client.Responses.New(ctx, *params, append(callUARequestOptions(call), callHeadersRequestOptions(call)...)...)
 780: 	if err != nil {
 781: 		return nil, toProviderErr(err)
 782: 	}
 783: 
 784: 	if response == nil {
 785: 		return nil, &fantasy.Error{Title: "no response", Message: "provider returned nil response"}
 786: 	}
 787: 
 788: 	if response.Error.Message != "" {
 789: 		return nil, &fantasy.Error{
 790: 			Title:   "provider error",
 791: 			Message: fmt.Sprintf("%s (code: %s)", response.Error.Message, response.Error.Code),
 792: 		}
 793: 	}
 794: 
 795: 	var content []fantasy.Content
 796: 	hasFunctionCall := false
 797: 	var pendingFunctionCalls []fantasy.ToolCallContent
 798: 
 799: 	for _, outputItem := range response.Output {
 800: 		switch outputItem.Type {
 801: 		case "message":
 802: 			for _, contentPart := range outputItem.Content {
 803: 				if contentPart.Type == "output_text" {
 804: 					content = append(content, fantasy.TextContent{
 805: 						Text: contentPart.Text,
 806: 					})
 807: 
 808: 					for _, annotation := range contentPart.Annotations {
 809: 						switch annotation.Type {
 810: 						case "url_citation":
 811: 							content = append(content, fantasy.SourceContent{
 812: 								SourceType: fantasy.SourceTypeURL,
 813: 								ID:         uuid.NewString(),
 814: 								URL:        annotation.URL,
 815: 								Title:      annotation.Title,
 816: 							})
 817: 						case "file_citation":
 818: 							title := "Document"
 819: 							if annotation.Filename != "" {
 820: 								title = annotation.Filename
 821: 							}
 822: 							filename := annotation.Filename
 823: 							if filename == "" {
 824: 								filename = annotation.FileID
 825: 							}
 826: 							content = append(content, fantasy.SourceContent{
 827: 								SourceType: fantasy.SourceTypeDocument,
 828: 								ID:         uuid.NewString(),
 829: 								MediaType:  "text/plain",
 830: 								Title:      title,
 831: 								Filename:   filename,
 832: 							})
 833: 						}
 834: 					}
 835: 				}
 836: 			}
 837: 
 838: 		case "function_call":
 839: 			hasFunctionCall = true
 840: 			pendingFunctionCalls = append(pendingFunctionCalls, fantasy.ToolCallContent{
 841: 				ProviderExecuted: false,
 842: 				ToolCallID:       outputItem.CallID,
 843: 				ToolName:         outputItem.Name,
 844: 				Input:            outputItem.Arguments.OfString,
 845: 			})
 846: 
 847: 		case "web_search_call":
 848: 			// Provider-executed web search tool call. Emit both
 849: 			// a ToolCallContent and ToolResultContent as a pair,
 850: 			// matching the vercel/ai pattern for provider tools.
 851: 			//
 852: 			// Note: source citations come from url_citation annotations
 853: 			// on the message text (handled in the "message" case above),
 854: 			// not from the web_search_call action.
 855: 			wsMeta := webSearchCallToMetadata(outputItem.ID, outputItem.Action)
 856: 			content = append(content, fantasy.ToolCallContent{
 857: 				ProviderExecuted: true,
 858: 				ToolCallID:       outputItem.ID,
 859: 				ToolName:         "web_search",
 860: 			})
 861: 			content = append(content, fantasy.ToolResultContent{
 862: 				ProviderExecuted: true,
 863: 				ToolCallID:       outputItem.ID,
 864: 				ToolName:         "web_search",
 865: 				ProviderMetadata: fantasy.ProviderMetadata{
 866: 					Name: wsMeta,
 867: 				},
 868: 			})
 869: 		case "reasoning":
 870: 			metadata := &ResponsesReasoningMetadata{
 871: 				ItemID: outputItem.ID,
 872: 			}
 873: 			if outputItem.EncryptedContent != "" {
 874: 				metadata.EncryptedContent = &outputItem.EncryptedContent
 875: 			}
 876: 
 877: 			if len(outputItem.Summary) == 0 && metadata.EncryptedContent == nil {
 878: 				continue
 879: 			}
 880: 
 881: 			// When there are no summary parts, add an empty reasoning part
 882: 			summaries := outputItem.Summary
 883: 			if len(summaries) == 0 {
 884: 				summaries = []responses.ResponseReasoningItemSummary{{Type: "summary_text", Text: ""}}
 885: 			}
 886: 
 887: 			for _, s := range summaries {
 888: 				metadata.Summary = append(metadata.Summary, s.Text)
 889: 			}
 890: 
 891: 			content = append(content, fantasy.ReasoningContent{
 892: 				Text: strings.Join(metadata.Summary, "\n"),
 893: 				ProviderMetadata: fantasy.ProviderMetadata{
 894: 					Name: metadata,
 895: 				},
 896: 			})
 897: 		}
 898: 	}
 899: 
 900: 	usage := responsesUsage(*response)
 901: 	finishReason := mapResponsesFinishReason(response.IncompleteDetails.Reason, hasFunctionCall)
 902: 	truncatedWithToolCalls := hasFunctionCall && finishReason == fantasy.FinishReasonLength
 903: 	if truncatedWithToolCalls {
 904: 		warnings = append(warnings, fantasy.CallWarning{
 905: 			Type:    fantasy.CallWarningTypeOther,
 906: 			Message: "tool calls were returned but the model hit the token limit; arguments may be truncated",
 907: 		})
 908: 	} else {
 909: 		for _, tc := range pendingFunctionCalls {
 910: 			content = append(content, tc)
 911: 		}
 912: 	}
 913: 
 914: 	return &fantasy.Response{
 915: 		Content:          content,
 916: 		Usage:            usage,
 917: 		FinishReason:     finishReason,
 918: 		ProviderMetadata: responsesProviderMetadata(response.ID),
 919: 		Warnings:         warnings,
 920: 	}, nil
 921: }
 922: 
 923: func mapResponsesFinishReason(reason string, hasFunctionCall bool) fantasy.FinishReason {
 924: 	if hasFunctionCall && reason != "max_tokens" && reason != "max_output_tokens" {
 925: 		return fantasy.FinishReasonToolCalls
 926: 	}
 927: 
 928: 	switch reason {
 929: 	case "":
 930: 		if hasFunctionCall {
 931: 			return fantasy.FinishReasonToolCalls
 932: 		}
 933: 		return fantasy.FinishReasonStop
 934: 	case "max_tokens", "max_output_tokens":
 935: 		return fantasy.FinishReasonLength
 936: 	case "content_filter":
 937: 		return fantasy.FinishReasonContentFilter
 938: 	default:
 939: 		return fantasy.FinishReasonOther
 940: 	}
 941: }
 942: 
 943: func (o responsesLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
 944: 	params, warnings, err := o.prepareParams(call)
 945: 	if err != nil {
 946: 		return nil, err
 947: 	}
 948: 
 949: 	stream := o.client.Responses.NewStreaming(ctx, *params, append(callUARequestOptions(call), callHeadersRequestOptions(call)...)...)
 950: 
 951: 	finishReason := fantasy.FinishReasonUnknown
 952: 	var usage fantasy.Usage
 953: 	// responseID tracks the server-assigned response ID. It's first set from the
 954: 	// response.created event and may be overwritten by response.completed or
 955: 	// response.incomplete events. Per the OpenAI API contract, these IDs are
 956: 	// identical; the overwrites ensure we have the final value even if an event
 957: 	// is missed.
 958: 	responseID := ""
 959: 	sawTerminalEvent := false
 960: 	ongoingToolCalls := make(map[int64]*ongoingToolCall)
 961: 	hasFunctionCall := false
 962: 	activeReasoning := make(map[string]*reasoningState)
 963: 
 964: 	return func(yield func(fantasy.StreamPart) bool) {
 965: 		if len(warnings) > 0 {
 966: 			if !yield(fantasy.StreamPart{
 967: 				Type:     fantasy.StreamPartTypeWarnings,
 968: 				Warnings: warnings,
 969: 			}) {
 970: 				return
 971: 			}
 972: 		}
 973: 
 974: 		for stream.Next() {
 975: 			event := stream.Current()
 976: 
 977: 			switch event.Type {
 978: 			case "response.created":
 979: 				created := event.AsResponseCreated()
 980: 				responseID = created.Response.ID
 981: 
 982: 			case "response.output_item.added":
 983: 				added := event.AsResponseOutputItemAdded()
 984: 				switch added.Item.Type {
 985: 				case "function_call":
 986: 					ongoingToolCalls[added.OutputIndex] = &ongoingToolCall{
 987: 						toolName:   added.Item.Name,
 988: 						toolCallID: added.Item.CallID,
 989: 					}
 990: 					if !yield(fantasy.StreamPart{
 991: 						Type:         fantasy.StreamPartTypeToolInputStart,
 992: 						ID:           added.Item.CallID,
 993: 						ToolCallName: added.Item.Name,
 994: 					}) {
 995: 						return
 996: 					}
 997: 
 998: 				case "web_search_call":
 999: 					// Provider-executed web search; emit start.
1000: 					if !yield(fantasy.StreamPart{
1001: 						Type:             fantasy.StreamPartTypeToolInputStart,
1002: 						ID:               added.Item.ID,
1003: 						ToolCallName:     "web_search",
1004: 						ProviderExecuted: true,
1005: 					}) {
1006: 						return
1007: 					}
1008: 
1009: 				case "message":
1010: 					if !yield(fantasy.StreamPart{
1011: 						Type: fantasy.StreamPartTypeTextStart,
1012: 						ID:   added.Item.ID,
1013: 					}) {
1014: 						return
1015: 					}
1016: 
1017: 				case "reasoning":
1018: 					metadata := &ResponsesReasoningMetadata{
1019: 						ItemID:  added.Item.ID,
1020: 						Summary: []string{},
1021: 					}
1022: 					if added.Item.EncryptedContent != "" {
1023: 						metadata.EncryptedContent = &added.Item.EncryptedContent
1024: 					}
1025: 
1026: 					activeReasoning[added.Item.ID] = &reasoningState{
1027: 						metadata: metadata,
1028: 					}
1029: 					if !yield(fantasy.StreamPart{
1030: 						Type: fantasy.StreamPartTypeReasoningStart,
1031: 						ID:   added.Item.ID,
1032: 						ProviderMetadata: fantasy.ProviderMetadata{
1033: 							Name: metadata,
1034: 						},
1035: 					}) {
1036: 						return
1037: 					}
1038: 				}
1039: 
1040: 			case "response.output_item.done":
1041: 				done := event.AsResponseOutputItemDone()
1042: 				switch done.Item.Type {
1043: 				case "function_call":
1044: 					tc := ongoingToolCalls[done.OutputIndex]
1045: 					if tc != nil {
1046: 						delete(ongoingToolCalls, done.OutputIndex)
1047: 						hasFunctionCall = true
1048: 
1049: 						if !yield(fantasy.StreamPart{
1050: 							Type: fantasy.StreamPartTypeToolInputEnd,
1051: 							ID:   done.Item.CallID,
1052: 						}) {
1053: 							return
1054: 						}
1055: 						if !yield(fantasy.StreamPart{
1056: 							Type:          fantasy.StreamPartTypeToolCall,
1057: 							ID:            done.Item.CallID,
1058: 							ToolCallName:  done.Item.Name,
1059: 							ToolCallInput: done.Item.Arguments.OfString,
1060: 						}) {
1061: 							return
1062: 						}
1063: 					}
1064: 
1065: 				case "web_search_call":
1066: 					// Provider-executed web search completed.
1067: 					// Source citations come from url_citation annotations
1068: 					// on the streamed message text, not from the action.
1069: 					if !yield(fantasy.StreamPart{
1070: 						Type: fantasy.StreamPartTypeToolInputEnd,
1071: 						ID:   done.Item.ID,
1072: 					}) {
1073: 						return
1074: 					}
1075: 					if !yield(fantasy.StreamPart{
1076: 						Type:             fantasy.StreamPartTypeToolCall,
1077: 						ID:               done.Item.ID,
1078: 						ToolCallName:     "web_search",
1079: 						ProviderExecuted: true,
1080: 					}) {
1081: 						return
1082: 					}
1083: 					// Emit a ToolResult so the agent framework
1084: 					// includes it in round-trip messages.
1085: 					if !yield(fantasy.StreamPart{
1086: 						Type:             fantasy.StreamPartTypeToolResult,
1087: 						ID:               done.Item.ID,
1088: 						ToolCallName:     "web_search",
1089: 						ProviderExecuted: true,
1090: 						ProviderMetadata: fantasy.ProviderMetadata{
1091: 							Name: webSearchCallToMetadata(done.Item.ID, done.Item.Action),
1092: 						},
1093: 					}) {
1094: 						return
1095: 					}
1096: 				case "message":
1097: 					if !yield(fantasy.StreamPart{
1098: 						Type: fantasy.StreamPartTypeTextEnd,
1099: 						ID:   done.Item.ID,
1100: 					}) {
1101: 						return
1102: 					}
1103: 
1104: 				case "reasoning":
1105: 					state := activeReasoning[done.Item.ID]
1106: 					if state != nil {
1107: 						if !yield(fantasy.StreamPart{
1108: 							Type: fantasy.StreamPartTypeReasoningEnd,
1109: 							ID:   done.Item.ID,
1110: 							ProviderMetadata: fantasy.ProviderMetadata{
1111: 								Name: state.metadata,
1112: 							},
1113: 						}) {
1114: 							return
1115: 						}
1116: 						delete(activeReasoning, done.Item.ID)
1117: 					}
1118: 				}
1119: 
1120: 			case "response.function_call_arguments.delta":
1121: 				delta := event.AsResponseFunctionCallArgumentsDelta()
1122: 				tc := ongoingToolCalls[delta.OutputIndex]
1123: 				if tc != nil {
1124: 					if !yield(fantasy.StreamPart{
1125: 						Type:  fantasy.StreamPartTypeToolInputDelta,
1126: 						ID:    tc.toolCallID,
1127: 						Delta: delta.Delta,
1128: 					}) {
1129: 						return
1130: 					}
1131: 				}
1132: 
1133: 			case "response.output_text.delta":
1134: 				textDelta := event.AsResponseOutputTextDelta()
1135: 				if !yield(fantasy.StreamPart{
1136: 					Type:  fantasy.StreamPartTypeTextDelta,
1137: 					ID:    textDelta.ItemID,
1138: 					Delta: textDelta.Delta,
1139: 				}) {
1140: 					return
1141: 				}
1142: 
1143: 			case "response.output_text.annotation.added":
1144: 				added := event.AsResponseOutputTextAnnotationAdded()
1145: 				// The Annotation field is typed as `any` in the SDK;
1146: 				// it deserializes as map[string]any from JSON.
1147: 				annotationMap, ok := added.Annotation.(map[string]any)
1148: 				if !ok {
1149: 					break
1150: 				}
1151: 				annotationType, _ := annotationMap["type"].(string)
1152: 				switch annotationType {
1153: 				case "url_citation":
1154: 					url, _ := annotationMap["url"].(string)
1155: 					title, _ := annotationMap["title"].(string)
1156: 					if !yield(fantasy.StreamPart{
1157: 						Type:       fantasy.StreamPartTypeSource,
1158: 						ID:         uuid.NewString(),
1159: 						SourceType: fantasy.SourceTypeURL,
1160: 						URL:        url,
1161: 						Title:      title,
1162: 					}) {
1163: 						return
1164: 					}
1165: 				case "file_citation":
1166: 					title := "Document"
1167: 					if fn, ok := annotationMap["filename"].(string); ok && fn != "" {
1168: 						title = fn
1169: 					}
1170: 					if !yield(fantasy.StreamPart{
1171: 						Type:       fantasy.StreamPartTypeSource,
1172: 						ID:         uuid.NewString(),
1173: 						SourceType: fantasy.SourceTypeDocument,
1174: 						Title:      title,
1175: 					}) {
1176: 						return
1177: 					}
1178: 				}
1179: 
1180: 			case "response.reasoning_summary_part.added":
1181: 				added := event.AsResponseReasoningSummaryPartAdded()
1182: 				state := activeReasoning[added.ItemID]
1183: 				if state != nil {
1184: 					state.metadata.Summary = append(state.metadata.Summary, "")
1185: 					activeReasoning[added.ItemID] = state
1186: 					if !yield(fantasy.StreamPart{
1187: 						Type:  fantasy.StreamPartTypeReasoningDelta,
1188: 						ID:    added.ItemID,
1189: 						Delta: "\n",
1190: 						ProviderMetadata: fantasy.ProviderMetadata{
1191: 							Name: state.metadata,
1192: 						},
1193: 					}) {
1194: 						return
1195: 					}
1196: 				}
1197: 
1198: 			case "response.reasoning_summary_text.delta":
1199: 				textDelta := event.AsResponseReasoningSummaryTextDelta()
1200: 				state := activeReasoning[textDelta.ItemID]
1201: 				if state != nil {
1202: 					if len(state.metadata.Summary)-1 >= int(textDelta.SummaryIndex) {
1203: 						state.metadata.Summary[textDelta.SummaryIndex] += textDelta.Delta
1204: 					}
1205: 					activeReasoning[textDelta.ItemID] = state
1206: 					if !yield(fantasy.StreamPart{
1207: 						Type:  fantasy.StreamPartTypeReasoningDelta,
1208: 						ID:    textDelta.ItemID,
1209: 						Delta: textDelta.Delta,
1210: 						ProviderMetadata: fantasy.ProviderMetadata{
1211: 							Name: state.metadata,
1212: 						},
1213: 					}) {
1214: 						return
1215: 					}
1216: 				}
1217: 
1218: 			case "response.completed":
1219: 				sawTerminalEvent = true
1220: 				completed := event.AsResponseCompleted()
1221: 				responseID = completed.Response.ID
1222: 				finishReason = mapResponsesFinishReason(completed.Response.IncompleteDetails.Reason, hasFunctionCall)
1223: 				usage = responsesUsage(completed.Response)
1224: 
1225: 			case "response.incomplete":
1226: 				sawTerminalEvent = true
1227: 				incomplete := event.AsResponseIncomplete()
1228: 				responseID = incomplete.Response.ID
1229: 				finishReason = mapResponsesFinishReason(incomplete.Response.IncompleteDetails.Reason, hasFunctionCall)
1230: 				usage = responsesUsage(incomplete.Response)
1231: 
1232: 			case "response.failed":
1233: 				failed := event.AsResponseFailed()
1234: 				if !yield(fantasy.StreamPart{
1235: 					Type:  fantasy.StreamPartTypeError,
1236: 					Error: responsesFailedStreamError(failed.Response.Error.Message, string(failed.Response.Error.Code)),
1237: 				}) {
1238: 					return
1239: 				}
1240: 				return
1241: 
1242: 			case "error":
1243: 				errorEvent := event.AsError()
1244: 				if !yield(fantasy.StreamPart{
1245: 					Type:  fantasy.StreamPartTypeError,
1246: 					Error: responsesErrorStreamError(errorEvent.Message, errorEvent.Code),
1247: 				}) {
1248: 					return
1249: 				}
1250: 				return
1251: 			}
1252: 		}
1253: 
1254: 		err := stream.Err()
1255: 		if err != nil && !errors.Is(err, io.EOF) {
1256: 			yield(fantasy.StreamPart{
1257: 				Type:  fantasy.StreamPartTypeError,
1258: 				Error: toProviderErr(err),
1259: 			})
1260: 			return
1261: 		}
1262: 
1263: 		if !sawTerminalEvent {
1264: 			err := ctx.Err()
1265: 			if err == nil {
1266: 				err = fantasy.NewIncompleteStreamError()
1267: 			}
1268: 			yield(fantasy.StreamPart{
1269: 				Type:  fantasy.StreamPartTypeError,
1270: 				Error: err,
1271: 			})
1272: 			return
1273: 		}
1274: 
1275: 		if hasFunctionCall && finishReason == fantasy.FinishReasonLength {
1276: 			yield(fantasy.StreamPart{
1277: 				Type: fantasy.StreamPartTypeWarnings,
1278: 				Warnings: []fantasy.CallWarning{{
1279: 					Type:    fantasy.CallWarningTypeOther,
1280: 					Message: "tool calls were returned but the model hit the token limit; arguments may be truncated",
1281: 				}},
1282: 			})
1283: 		}
1284: 
1285: 		yield(fantasy.StreamPart{
1286: 			Type:             fantasy.StreamPartTypeFinish,
1287: 			Usage:            usage,
1288: 			FinishReason:     finishReason,
1289: 			ProviderMetadata: responsesProviderMetadata(responseID),
1290: 		})
1291: 	}, nil
1292: }
1293: 
1294: // responsesFailedStreamError intentionally returns a provider-declared failure
1295: // instead of a retryable transport error. Only synthetic stream truncation
1296: // errors are wrapped with io.ErrUnexpectedEOF.
1297: func responsesFailedStreamError(message, code string) error {
1298: 	return responsesStreamFailureError("response failed", message, code)
1299: }
1300: 
1301: func responsesErrorStreamError(message, code string) error {
1302: 	return responsesStreamFailureError("response error", message, code)
1303: }
1304: 
1305: func responsesStreamFailureError(title, message, code string) error {
1306: 	if code != "" {
1307: 		message = fmt.Sprintf("%s (code: %s)", message, code)
1308: 	}
1309: 	return &fantasy.Error{Title: title, Message: message}
1310: }
1311: 
1312: // toWebSearchToolParam converts a ProviderDefinedTool with ID
1313: // "web_search" into the OpenAI SDK's WebSearchToolParam.
1314: func toWebSearchToolParam(pt fantasy.ProviderDefinedTool) responses.ToolUnionParam {
1315: 	wst := responses.WebSearchToolParam{
1316: 		Type: responses.WebSearchToolTypeWebSearch,
1317: 	}
1318: 	if pt.Args != nil {
1319: 		if size, ok := pt.Args["search_context_size"].(SearchContextSize); ok && size != "" {
1320: 			wst.SearchContextSize = responses.WebSearchToolSearchContextSize(size)
1321: 		}
1322: 		// Also accept plain string for search_context_size.
1323: 		if size, ok := pt.Args["search_context_size"].(string); ok && size != "" {
1324: 			wst.SearchContextSize = responses.WebSearchToolSearchContextSize(size)
1325: 		}
1326: 		if domains, ok := pt.Args["allowed_domains"].([]string); ok && len(domains) > 0 {
1327: 			wst.Filters.AllowedDomains = domains
1328: 		}
1329: 		if loc, ok := pt.Args["user_location"].(*WebSearchUserLocation); ok && loc != nil {
1330: 			if loc.City != "" {
1331: 				wst.UserLocation.City = param.NewOpt(loc.City)
1332: 			}
1333: 			if loc.Region != "" {
1334: 				wst.UserLocation.Region = param.NewOpt(loc.Region)
1335: 			}
1336: 			if loc.Country != "" {
1337: 				wst.UserLocation.Country = param.NewOpt(loc.Country)
1338: 			}
1339: 			if loc.Timezone != "" {
1340: 				wst.UserLocation.Timezone = param.NewOpt(loc.Timezone)
1341: 			}
1342: 		}
1343: 	}
1344: 	return responses.ToolUnionParam{
1345: 		OfWebSearch: &wst,
1346: 	}
1347: }
1348: 
1349: // webSearchCallToMetadata converts an OpenAI web search call output
1350: // into our structured metadata for round-tripping.
1351: func webSearchCallToMetadata(itemID string, action responses.ResponseOutputItemUnionAction) *WebSearchCallMetadata {
1352: 	meta := &WebSearchCallMetadata{ItemID: itemID}
1353: 	if action.Type != "" {
1354: 		a := &WebSearchAction{
1355: 			Type:  action.Type,
1356: 			Query: action.Query,
1357: 		}
1358: 		for _, src := range action.Sources {
1359: 			a.Sources = append(a.Sources, WebSearchSource{
1360: 				Type: string(src.Type),
1361: 				URL:  src.URL,
1362: 			})
1363: 		}
1364: 		meta.Action = a
1365: 	}
1366: 	return meta
1367: }
1368: 
1369: // GetReasoningMetadata extracts reasoning metadata from provider options for responses models.
1370: func GetReasoningMetadata(providerOptions fantasy.ProviderOptions) *ResponsesReasoningMetadata {
1371: 	if openaiResponsesOptions, ok := providerOptions[Name]; ok {
1372: 		if reasoning, ok := openaiResponsesOptions.(*ResponsesReasoningMetadata); ok {
1373: 			return reasoning
1374: 		}
1375: 	}
1376: 	return nil
1377: }
1378: 
1379: type ongoingToolCall struct {
1380: 	toolName   string
1381: 	toolCallID string
1382: }
1383: 
1384: type reasoningState struct {
1385: 	metadata *ResponsesReasoningMetadata
1386: }
1387: 
1388: // GenerateObject implements fantasy.LanguageModel.
1389: func (o responsesLanguageModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
1390: 	switch o.objectMode {
1391: 	case fantasy.ObjectModeText:
1392: 		return object.GenerateWithText(ctx, o, call)
1393: 	case fantasy.ObjectModeTool:
1394: 		return object.GenerateWithTool(ctx, o, call)
1395: 	default:
1396: 		return o.generateObjectWithJSONMode(ctx, call)
1397: 	}
1398: }
1399: 
1400: // StreamObject implements fantasy.LanguageModel.
1401: func (o responsesLanguageModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
1402: 	switch o.objectMode {
1403: 	case fantasy.ObjectModeTool:
1404: 		return object.StreamWithTool(ctx, o, call)
1405: 	case fantasy.ObjectModeText:
1406: 		return object.StreamWithText(ctx, o, call)
1407: 	default:
1408: 		return o.streamObjectWithJSONMode(ctx, call)
1409: 	}
1410: }
1411: 
1412: func (o responsesLanguageModel) generateObjectWithJSONMode(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
1413: 	// Convert our Schema to OpenAI's JSON Schema format
1414: 	jsonSchemaMap := schema.ToMap(call.Schema)
1415: 
1416: 	// Add additionalProperties: false recursively for strict mode (OpenAI requirement)
1417: 	addAdditionalPropertiesFalse(jsonSchemaMap)
1418: 
1419: 	schemaName := call.SchemaName
1420: 	if schemaName == "" {
1421: 		schemaName = "response"
1422: 	}
1423: 
1424: 	// Build request using prepareParams
1425: 	fantasyCall := fantasy.Call{
1426: 		Prompt:           call.Prompt,
1427: 		MaxOutputTokens:  call.MaxOutputTokens,
1428: 		Temperature:      call.Temperature,
1429: 		TopP:             call.TopP,
1430: 		PresencePenalty:  call.PresencePenalty,
1431: 		FrequencyPenalty: call.FrequencyPenalty,
1432: 		ProviderOptions:  call.ProviderOptions,
1433: 	}
1434: 
1435: 	params, warnings, err := o.prepareParams(fantasyCall)
1436: 	if err != nil {
1437: 		return nil, err
1438: 	}
1439: 
1440: 	// Add structured output via Text.Format field
1441: 	params.Text = responses.ResponseTextConfigParam{
1442: 		Format: responses.ResponseFormatTextConfigParamOfJSONSchema(schemaName, jsonSchemaMap),
1443: 	}
1444: 
1445: 	// Make request
1446: 	response, err := o.client.Responses.New(ctx, *params, append(objectCallUARequestOptions(call), objectCallHeadersRequestOptions(call)...)...)
1447: 	if err != nil {
1448: 		return nil, toProviderErr(err)
1449: 	}
1450: 
1451: 	if response.Error.Message != "" {
1452: 		return nil, &fantasy.Error{
1453: 			Title:   "provider error",
1454: 			Message: fmt.Sprintf("%s (code: %s)", response.Error.Message, response.Error.Code),
1455: 		}
1456: 	}
1457: 
1458: 	// Extract JSON text from response
1459: 	var jsonText string
1460: 	for _, outputItem := range response.Output {
1461: 		if outputItem.Type == "message" {
1462: 			for _, contentPart := range outputItem.Content {
1463: 				if contentPart.Type == "output_text" {
1464: 					jsonText = contentPart.Text
1465: 					break
1466: 				}
1467: 			}
1468: 		}
1469: 	}
1470: 
1471: 	if jsonText == "" {
1472: 		usage := fantasy.Usage{
1473: 			InputTokens:  response.Usage.InputTokens,
1474: 			OutputTokens: response.Usage.OutputTokens,
1475: 			TotalTokens:  response.Usage.InputTokens + response.Usage.OutputTokens,
1476: 		}
1477: 		finishReason := mapResponsesFinishReason(response.IncompleteDetails.Reason, false)
1478: 		return nil, &fantasy.NoObjectGeneratedError{
1479: 			RawText:      "",
1480: 			ParseError:   fmt.Errorf("no text content in response"),
1481: 			Usage:        usage,
1482: 			FinishReason: finishReason,
1483: 		}
1484: 	}
1485: 
1486: 	// Parse and validate
1487: 	var obj any
1488: 	if call.RepairText != nil {
1489: 		obj, err = schema.ParseAndValidateWithRepair(ctx, jsonText, call.Schema, call.RepairText)
1490: 	} else {
1491: 		obj, err = schema.ParseAndValidate(jsonText, call.Schema)
1492: 	}
1493: 
1494: 	usage := responsesUsage(*response)
1495: 	finishReason := mapResponsesFinishReason(response.IncompleteDetails.Reason, false)
1496: 
1497: 	if err != nil {
1498: 		// Add usage info to error
1499: 		if nogErr, ok := err.(*fantasy.NoObjectGeneratedError); ok {
1500: 			nogErr.Usage = usage
1501: 			nogErr.FinishReason = finishReason
1502: 		}
1503: 		return nil, err
1504: 	}
1505: 
1506: 	return &fantasy.ObjectResponse{
1507: 		Object:           obj,
1508: 		RawText:          jsonText,
1509: 		Usage:            usage,
1510: 		FinishReason:     finishReason,
1511: 		Warnings:         warnings,
1512: 		ProviderMetadata: responsesProviderMetadata(response.ID),
1513: 	}, nil
1514: }
1515: 
1516: func (o responsesLanguageModel) streamObjectWithJSONMode(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
1517: 	// Convert our Schema to OpenAI's JSON Schema format
1518: 	jsonSchemaMap := schema.ToMap(call.Schema)
1519: 
1520: 	// Add additionalProperties: false recursively for strict mode (OpenAI requirement)
1521: 	addAdditionalPropertiesFalse(jsonSchemaMap)
1522: 
1523: 	schemaName := call.SchemaName
1524: 	if schemaName == "" {
1525: 		schemaName = "response"
1526: 	}
1527: 
1528: 	// Build request using prepareParams
1529: 	fantasyCall := fantasy.Call{
1530: 		Prompt:           call.Prompt,
1531: 		MaxOutputTokens:  call.MaxOutputTokens,
1532: 		Temperature:      call.Temperature,
1533: 		TopP:             call.TopP,
1534: 		PresencePenalty:  call.PresencePenalty,
1535: 		FrequencyPenalty: call.FrequencyPenalty,
1536: 		ProviderOptions:  call.ProviderOptions,
1537: 	}
1538: 
1539: 	params, warnings, err := o.prepareParams(fantasyCall)
1540: 	if err != nil {
1541: 		return nil, err
1542: 	}
1543: 
1544: 	// Add structured output via Text.Format field
1545: 	params.Text = responses.ResponseTextConfigParam{
1546: 		Format: responses.ResponseFormatTextConfigParamOfJSONSchema(schemaName, jsonSchemaMap),
1547: 	}
1548: 
1549: 	stream := o.client.Responses.NewStreaming(ctx, *params, append(objectCallUARequestOptions(call), objectCallHeadersRequestOptions(call)...)...)
1550: 
1551: 	return func(yield func(fantasy.ObjectStreamPart) bool) {
1552: 		if len(warnings) > 0 {
1553: 			if !yield(fantasy.ObjectStreamPart{
1554: 				Type:     fantasy.ObjectStreamPartTypeObject,
1555: 				Warnings: warnings,
1556: 			}) {
1557: 				return
1558: 			}
1559: 		}
1560: 
1561: 		var accumulated string
1562: 		var lastParsedObject any
1563: 		var usage fantasy.Usage
1564: 		var finishReason fantasy.FinishReason
1565: 		// responseID tracks the server-assigned response ID. It's first set from the
1566: 		// response.created event and may be overwritten by response.completed or
1567: 		// response.incomplete events. Per the OpenAI API contract, these IDs are
1568: 		// identical; the overwrites ensure we have the final value even if an event
1569: 		// is missed.
1570: 		var responseID string
1571: 		var sawTerminalEvent bool
1572: 		hasFunctionCall := false
1573: 
1574: 		for stream.Next() {
1575: 			event := stream.Current()
1576: 
1577: 			switch event.Type {
1578: 			case "response.created":
1579: 				created := event.AsResponseCreated()
1580: 				responseID = created.Response.ID
1581: 
1582: 			case "response.output_text.delta":
1583: 				textDelta := event.AsResponseOutputTextDelta()
1584: 				accumulated += textDelta.Delta
1585: 
1586: 				// Try to parse the accumulated text
1587: 				obj, state, parseErr := schema.ParsePartialJSON(accumulated)
1588: 
1589: 				// If we successfully parsed, validate and emit
1590: 				if state == schema.ParseStateSuccessful || state == schema.ParseStateRepaired {
1591: 					if err := schema.ValidateAgainstSchema(obj, call.Schema); err == nil {
1592: 						// Only emit if object is different from last
1593: 						if !reflect.DeepEqual(obj, lastParsedObject) {
1594: 							if !yield(fantasy.ObjectStreamPart{
1595: 								Type:   fantasy.ObjectStreamPartTypeObject,
1596: 								Object: obj,
1597: 							}) {
1598: 								return
1599: 							}
1600: 							lastParsedObject = obj
1601: 						}
1602: 					}
1603: 				}
1604: 
1605: 				// If parsing failed and we have a repair function, try it
1606: 				if state == schema.ParseStateFailed && call.RepairText != nil {
1607: 					repairedText, repairErr := call.RepairText(ctx, accumulated, parseErr)
1608: 					if repairErr == nil {
1609: 						obj2, state2, _ := schema.ParsePartialJSON(repairedText)
1610: 						if (state2 == schema.ParseStateSuccessful || state2 == schema.ParseStateRepaired) &&
1611: 							schema.ValidateAgainstSchema(obj2, call.Schema) == nil {
1612: 							if !reflect.DeepEqual(obj2, lastParsedObject) {
1613: 								if !yield(fantasy.ObjectStreamPart{
1614: 									Type:   fantasy.ObjectStreamPartTypeObject,
1615: 									Object: obj2,
1616: 								}) {
1617: 									return
1618: 								}
1619: 								lastParsedObject = obj2
1620: 							}
1621: 						}
1622: 					}
1623: 				}
1624: 
1625: 			case "response.completed":
1626: 				sawTerminalEvent = true
1627: 				completed := event.AsResponseCompleted()
1628: 				responseID = completed.Response.ID
1629: 				finishReason = mapResponsesFinishReason(completed.Response.IncompleteDetails.Reason, hasFunctionCall)
1630: 				usage = responsesUsage(completed.Response)
1631: 
1632: 			case "response.incomplete":
1633: 				sawTerminalEvent = true
1634: 				incomplete := event.AsResponseIncomplete()
1635: 				responseID = incomplete.Response.ID
1636: 				finishReason = mapResponsesFinishReason(incomplete.Response.IncompleteDetails.Reason, hasFunctionCall)
1637: 				usage = responsesUsage(incomplete.Response)
1638: 
1639: 			case "response.failed":
1640: 				failed := event.AsResponseFailed()
1641: 				if !yield(fantasy.ObjectStreamPart{
1642: 					Type:  fantasy.ObjectStreamPartTypeError,
1643: 					Error: responsesFailedStreamError(failed.Response.Error.Message, string(failed.Response.Error.Code)),
1644: 				}) {
1645: 					return
1646: 				}
1647: 				return
1648: 
1649: 			case "error":
1650: 				errorEvent := event.AsError()
1651: 				if !yield(fantasy.ObjectStreamPart{
1652: 					Type:  fantasy.ObjectStreamPartTypeError,
1653: 					Error: responsesErrorStreamError(errorEvent.Message, errorEvent.Code),
1654: 				}) {
1655: 					return
1656: 				}
1657: 				return
1658: 			}
1659: 		}
1660: 
1661: 		err := stream.Err()
1662: 		if err != nil && !errors.Is(err, io.EOF) {
1663: 			yield(fantasy.ObjectStreamPart{
1664: 				Type:  fantasy.ObjectStreamPartTypeError,
1665: 				Error: toProviderErr(err),
1666: 			})
1667: 			return
1668: 		}
1669: 
1670: 		if !sawTerminalEvent {
1671: 			err := ctx.Err()
1672: 			if err == nil {
1673: 				err = fantasy.NewIncompleteStreamError()
1674: 			}
1675: 			yield(fantasy.ObjectStreamPart{
1676: 				Type:  fantasy.ObjectStreamPartTypeError,
1677: 				Error: err,
1678: 			})
1679: 			return
1680: 		}
1681: 
1682: 		// Final validation and emit
1683: 		if lastParsedObject != nil {
1684: 			yield(fantasy.ObjectStreamPart{
1685: 				Type:             fantasy.ObjectStreamPartTypeFinish,
1686: 				Usage:            usage,
1687: 				FinishReason:     finishReason,
1688: 				ProviderMetadata: responsesProviderMetadata(responseID),
1689: 			})
1690: 		} else {
1691: 			// No object was generated
1692: 			yield(fantasy.ObjectStreamPart{
1693: 				Type: fantasy.ObjectStreamPartTypeError,
1694: 				Error: &fantasy.NoObjectGeneratedError{
1695: 					RawText:      accumulated,
1696: 					ParseError:   fmt.Errorf("no valid object generated in stream"),
1697: 					Usage:        usage,
1698: 					FinishReason: finishReason,
1699: 				},
1700: 			})
1701: 		}
1702: 	}, nil
1703: }
```

## File: providers/openai/responses_options.go
```go
  1: // Package openai provides an implementation of the fantasy AI SDK for OpenAI's language models.
  2: package openai
  3: 
  4: import (
  5: 	"encoding/json"
  6: 	"slices"
  7: 	"strings"
  8: 
  9: 	"charm.land/fantasy"
 10: )
 11: 
 12: // Global type identifiers for OpenAI Responses API-specific data.
 13: const (
 14: 	TypeResponsesProviderMetadata  = Name + ".responses.metadata"
 15: 	TypeResponsesProviderOptions   = Name + ".responses.options"
 16: 	TypeResponsesReasoningMetadata = Name + ".responses.reasoning_metadata"
 17: 	TypeWebSearchCallMetadata      = Name + ".responses.web_search_call_metadata"
 18: )
 19: 
 20: // Register OpenAI Responses API-specific types with the global registry.
 21: func init() {
 22: 	fantasy.RegisterProviderType(TypeResponsesProviderMetadata, func(data []byte) (fantasy.ProviderOptionsData, error) {
 23: 		var v ResponsesProviderMetadata
 24: 		if err := json.Unmarshal(data, &v); err != nil {
 25: 			return nil, err
 26: 		}
 27: 		return &v, nil
 28: 	})
 29: 	fantasy.RegisterProviderType(TypeResponsesProviderOptions, func(data []byte) (fantasy.ProviderOptionsData, error) {
 30: 		var v ResponsesProviderOptions
 31: 		if err := json.Unmarshal(data, &v); err != nil {
 32: 			return nil, err
 33: 		}
 34: 		return &v, nil
 35: 	})
 36: 	fantasy.RegisterProviderType(TypeResponsesReasoningMetadata, func(data []byte) (fantasy.ProviderOptionsData, error) {
 37: 		var v ResponsesReasoningMetadata
 38: 		if err := json.Unmarshal(data, &v); err != nil {
 39: 			return nil, err
 40: 		}
 41: 		return &v, nil
 42: 	})
 43: 	fantasy.RegisterProviderType(TypeWebSearchCallMetadata, func(data []byte) (fantasy.ProviderOptionsData, error) {
 44: 		var v WebSearchCallMetadata
 45: 		if err := json.Unmarshal(data, &v); err != nil {
 46: 			return nil, err
 47: 		}
 48: 		return &v, nil
 49: 	})
 50: }
 51: 
 52: // ResponsesProviderMetadata contains response-level metadata from the OpenAI Responses API.
 53: // The ResponseID can be used as PreviousResponseID in follow-up requests to chain responses.
 54: type ResponsesProviderMetadata struct {
 55: 	ResponseID string `json:"response_id"`
 56: }
 57: 
 58: var _ fantasy.ProviderOptionsData = (*ResponsesProviderMetadata)(nil)
 59: 
 60: // Options implements the ProviderOptions interface.
 61: func (*ResponsesProviderMetadata) Options() {}
 62: 
 63: // MarshalJSON implements custom JSON marshaling with type info for ResponsesProviderMetadata.
 64: func (m ResponsesProviderMetadata) MarshalJSON() ([]byte, error) {
 65: 	type plain ResponsesProviderMetadata
 66: 	return fantasy.MarshalProviderType(TypeResponsesProviderMetadata, plain(m))
 67: }
 68: 
 69: // UnmarshalJSON implements custom JSON unmarshaling with type info for ResponsesProviderMetadata.
 70: func (m *ResponsesProviderMetadata) UnmarshalJSON(data []byte) error {
 71: 	type plain ResponsesProviderMetadata
 72: 	var p plain
 73: 	if err := fantasy.UnmarshalProviderType(data, &p); err != nil {
 74: 		return err
 75: 	}
 76: 	*m = ResponsesProviderMetadata(p)
 77: 	return nil
 78: }
 79: 
 80: // ResponsesReasoningMetadata represents reasoning metadata for OpenAI Responses API.
 81: type ResponsesReasoningMetadata struct {
 82: 	ItemID           string   `json:"item_id"`
 83: 	EncryptedContent *string  `json:"encrypted_content"`
 84: 	Summary          []string `json:"summary"`
 85: }
 86: 
 87: // Options implements the ProviderOptions interface.
 88: func (*ResponsesReasoningMetadata) Options() {}
 89: 
 90: // MarshalJSON implements custom JSON marshaling with type info for ResponsesReasoningMetadata.
 91: func (m ResponsesReasoningMetadata) MarshalJSON() ([]byte, error) {
 92: 	type plain ResponsesReasoningMetadata
 93: 	return fantasy.MarshalProviderType(TypeResponsesReasoningMetadata, plain(m))
 94: }
 95: 
 96: // UnmarshalJSON implements custom JSON unmarshaling with type info for ResponsesReasoningMetadata.
 97: func (m *ResponsesReasoningMetadata) UnmarshalJSON(data []byte) error {
 98: 	type plain ResponsesReasoningMetadata
 99: 	var p plain
100: 	if err := fantasy.UnmarshalProviderType(data, &p); err != nil {
101: 		return err
102: 	}
103: 	*m = ResponsesReasoningMetadata(p)
104: 	return nil
105: }
106: 
107: // IncludeType represents the type of content to include for OpenAI Responses API.
108: type IncludeType string
109: 
110: const (
111: 	// IncludeReasoningEncryptedContent includes encrypted reasoning content.
112: 	IncludeReasoningEncryptedContent IncludeType = "reasoning.encrypted_content"
113: 	// IncludeFileSearchCallResults includes file search call results.
114: 	IncludeFileSearchCallResults IncludeType = "file_search_call.results"
115: 	// IncludeMessageOutputTextLogprobs includes message output text log probabilities.
116: 	IncludeMessageOutputTextLogprobs IncludeType = "message.output_text.logprobs"
117: )
118: 
119: // ServiceTier represents the service tier for OpenAI Responses API.
120: type ServiceTier string
121: 
122: const (
123: 	// ServiceTierAuto represents the auto service tier.
124: 	ServiceTierAuto ServiceTier = "auto"
125: 	// ServiceTierFlex represents the flex service tier.
126: 	ServiceTierFlex ServiceTier = "flex"
127: 	// ServiceTierPriority represents the priority service tier.
128: 	ServiceTierPriority ServiceTier = "priority"
129: )
130: 
131: // TextVerbosity represents the text verbosity level for OpenAI Responses API.
132: type TextVerbosity string
133: 
134: const (
135: 	// TextVerbosityLow represents low text verbosity.
136: 	TextVerbosityLow TextVerbosity = "low"
137: 	// TextVerbosityMedium represents medium text verbosity.
138: 	TextVerbosityMedium TextVerbosity = "medium"
139: 	// TextVerbosityHigh represents high text verbosity.
140: 	TextVerbosityHigh TextVerbosity = "high"
141: )
142: 
143: // ResponsesProviderOptions represents additional options for OpenAI Responses API.
144: type ResponsesProviderOptions struct {
145: 	Include           []IncludeType  `json:"include"`
146: 	Instructions      *string        `json:"instructions"`
147: 	Logprobs          any            `json:"logprobs"`
148: 	MaxToolCalls      *int64         `json:"max_tool_calls"`
149: 	Metadata          map[string]any `json:"metadata"`
150: 	ParallelToolCalls *bool          `json:"parallel_tool_calls"`
151: 	// PreviousResponseID chains this request to a prior stored response, enabling
152: 	// server-side conversation state. When set, the prompt should contain only the
153: 	// new incremental turn—not replayed assistant history.
154: 	PreviousResponseID *string          `json:"previous_response_id"`
155: 	PromptCacheKey     *string          `json:"prompt_cache_key"`
156: 	ReasoningEffort    *ReasoningEffort `json:"reasoning_effort"`
157: 	ReasoningSummary   *string          `json:"reasoning_summary"`
158: 	SafetyIdentifier   *string          `json:"safety_identifier"`
159: 	ServiceTier        *ServiceTier     `json:"service_tier"`
160: 	// Store indicates whether OpenAI should persist this response for future
161: 	// retrieval and chaining via PreviousResponseID. Defaults to false to prevent
162: 	// unintended storage of potentially sensitive conversations.
163: 	Store            *bool          `json:"store"`
164: 	StrictJSONSchema *bool          `json:"strict_json_schema"`
165: 	TextVerbosity    *TextVerbosity `json:"text_verbosity"`
166: 	User             *string        `json:"user"`
167: }
168: 
169: // Options implements the ProviderOptions interface.
170: func (*ResponsesProviderOptions) Options() {}
171: 
172: // MarshalJSON implements custom JSON marshaling with type info for ResponsesProviderOptions.
173: func (o ResponsesProviderOptions) MarshalJSON() ([]byte, error) {
174: 	type plain ResponsesProviderOptions
175: 	return fantasy.MarshalProviderType(TypeResponsesProviderOptions, plain(o))
176: }
177: 
178: // UnmarshalJSON implements custom JSON unmarshaling with type info for ResponsesProviderOptions.
179: func (o *ResponsesProviderOptions) UnmarshalJSON(data []byte) error {
180: 	type plain ResponsesProviderOptions
181: 	var p plain
182: 	if err := fantasy.UnmarshalProviderType(data, &p); err != nil {
183: 		return err
184: 	}
185: 	*o = ResponsesProviderOptions(p)
186: 	return nil
187: }
188: 
189: // responsesReasoningModelIds lists the model IDs that support reasoning for OpenAI Responses API.
190: var responsesReasoningModelIDs = []string{
191: 	"o1",
192: 	"o1-2024-12-17",
193: 	"o3-mini",
194: 	"o3-mini-2025-01-31",
195: 	"o3",
196: 	"o3-2025-04-16",
197: 	"o4-mini",
198: 	"o4-mini-2025-04-16",
199: 	"codex-mini-latest",
200: 	"gpt-5",
201: 	"gpt-5-2025-08-07",
202: 	"gpt-5-mini",
203: 	"gpt-5-mini-2025-08-07",
204: 	"gpt-5-nano",
205: 	"gpt-5-nano-2025-08-07",
206: 	"gpt-5-codex",
207: 	"gpt-5-chat",
208: 	"gpt-5-pro",
209: 	"gpt-5.1",
210: 	"gpt-5.1-codex",
211: 	"gpt-5.1-codex-max",
212: 	"gpt-5.1-codex-mini",
213: 	"gpt-5.1-chat",
214: 	"gpt-5.2",
215: 	"gpt-5.2-codex",
216: 	"gpt-5.3",
217: 	"gpt-5.3-codex",
218: 	"gpt-5.4",
219: 	"gpt-5.4-pro",
220: 	"gpt-5.4-mini",
221: 	"gpt-5.4-nano",
222: 	"gpt-5.4-codex",
223: 	"gpt-5.5",
224: 	"gpt-5.5-pro",
225: 	"gpt-5.5-mini",
226: 	"gpt-5.5-nano",
227: 	"gpt-5.5-codex",
228: 	"gpt-5.6",
229: 	"gpt-5.6-pro",
230: 	"gpt-5.6-mini",
231: 	"gpt-5.6-nano",
232: 	"gpt-5.6-codex",
233: 	"gpt-oss-120b",
234: }
235: 
236: // responsesModelIds lists all model IDs for OpenAI Responses API.
237: var responsesModelIDs = append([]string{
238: 	"gpt-4.1",
239: 	"gpt-4.1-2025-04-14",
240: 	"gpt-4.1-mini",
241: 	"gpt-4.1-mini-2025-04-14",
242: 	"gpt-4.1-nano",
243: 	"gpt-4.1-nano-2025-04-14",
244: 	"gpt-4o",
245: 	"gpt-4o-2024-05-13",
246: 	"gpt-4o-2024-08-06",
247: 	"gpt-4o-2024-11-20",
248: 	"gpt-4o-mini",
249: 	"gpt-4o-mini-2024-07-18",
250: 	"gpt-4-turbo",
251: 	"gpt-4-turbo-2024-04-09",
252: 	"gpt-4-turbo-preview",
253: 	"gpt-4-0125-preview",
254: 	"gpt-4-1106-preview",
255: 	"gpt-4",
256: 	"gpt-4-0613",
257: 	"gpt-4.5-preview",
258: 	"gpt-4.5-preview-2025-02-27",
259: 	"gpt-3.5-turbo-0125",
260: 	"gpt-3.5-turbo",
261: 	"gpt-3.5-turbo-1106",
262: 	"chatgpt-4o-latest",
263: 	"gpt-5-chat-latest",
264: }, responsesReasoningModelIDs...)
265: 
266: // NewResponsesProviderOptions creates new provider options for OpenAI Responses API.
267: func NewResponsesProviderOptions(opts *ResponsesProviderOptions) fantasy.ProviderOptions {
268: 	return fantasy.ProviderOptions{
269: 		Name: opts,
270: 	}
271: }
272: 
273: // ParseResponsesOptions parses provider options from a map for OpenAI Responses API.
274: func ParseResponsesOptions(data map[string]any) (*ResponsesProviderOptions, error) {
275: 	var options ResponsesProviderOptions
276: 	if err := fantasy.ParseOptions(data, &options); err != nil {
277: 		return nil, err
278: 	}
279: 	return &options, nil
280: }
281: 
282: // IsResponsesModel checks if a model ID is a Responses API model for OpenAI.
283: func IsResponsesModel(modelID string) bool {
284: 	return slices.Contains(responsesModelIDs, modelID) ||
285: 		strings.Contains(strings.ToLower(modelID), "gpt-4") ||
286: 		strings.Contains(strings.ToLower(modelID), "gpt-5")
287: }
288: 
289: // IsResponsesReasoningModel checks if a model ID is a Responses API reasoning model for OpenAI.
290: func IsResponsesReasoningModel(modelID string) bool {
291: 	return slices.Contains(responsesReasoningModelIDs, modelID) ||
292: 		strings.Contains(strings.ToLower(modelID), "gpt-4") ||
293: 		strings.Contains(strings.ToLower(modelID), "gpt-5")
294: }
295: 
296: // SearchContextSize controls how much context window space the
297: // web search tool uses. Maps to the OpenAI API's
298: // search_context_size parameter.
299: type SearchContextSize string
300: 
301: const (
302: 	// SearchContextSizeLow uses minimal context for search results.
303: 	SearchContextSizeLow SearchContextSize = "low"
304: 	// SearchContextSizeMedium is the default context size.
305: 	SearchContextSizeMedium SearchContextSize = "medium"
306: 	// SearchContextSizeHigh uses maximal context for search results.
307: 	SearchContextSizeHigh SearchContextSize = "high"
308: )
309: 
310: // WebSearchUserLocation provides geographic context for more
311: // relevant web search results.
312: type WebSearchUserLocation struct {
313: 	City     string `json:"city,omitempty"`
314: 	Region   string `json:"region,omitempty"`
315: 	Country  string `json:"country,omitempty"`
316: 	Timezone string `json:"timezone,omitempty"`
317: }
318: 
319: // WebSearchToolOptions configures the OpenAI web search tool.
320: type WebSearchToolOptions struct {
321: 	// SearchContextSize controls the amount of context window
322: 	// space used for search results. Defaults to medium.
323: 	SearchContextSize SearchContextSize
324: 	// AllowedDomains restricts search results to these domains.
325: 	// Subdomains are included automatically.
326: 	AllowedDomains []string
327: 	// UserLocation provides geographic context for more
328: 	// relevant search results.
329: 	UserLocation *WebSearchUserLocation
330: }
331: 
332: // WebSearchTool creates a provider-defined web search tool for
333: // OpenAI models. Pass nil for default options.
334: func WebSearchTool(opts *WebSearchToolOptions) fantasy.ProviderDefinedTool {
335: 	tool := fantasy.ProviderDefinedTool{
336: 		ID:   "web_search",
337: 		Name: "web_search",
338: 	}
339: 	if opts == nil {
340: 		return tool
341: 	}
342: 	args := map[string]any{}
343: 	if opts.SearchContextSize != "" {
344: 		args["search_context_size"] = opts.SearchContextSize
345: 	}
346: 	if len(opts.AllowedDomains) > 0 {
347: 		args["allowed_domains"] = opts.AllowedDomains
348: 	}
349: 	if opts.UserLocation != nil {
350: 		args["user_location"] = opts.UserLocation
351: 	}
352: 	if len(args) > 0 {
353: 		tool.Args = args
354: 	}
355: 	return tool
356: }
357: 
358: // WebSearchSource represents a single source from a web search action.
359: type WebSearchSource struct {
360: 	Type string `json:"type"`
361: 	URL  string `json:"url"`
362: }
363: 
364: // WebSearchAction represents the action taken during a web search call.
365: type WebSearchAction struct {
366: 	// Type is the kind of action: "search", "open_page", or "find".
367: 	Type string `json:"type"`
368: 	// Query is the search query (present when Type is "search").
369: 	Query string `json:"query,omitempty"`
370: 	// Sources are the results returned by the search.
371: 	Sources []WebSearchSource `json:"sources,omitempty"`
372: }
373: 
374: // WebSearchCallMetadata stores structured data from a web_search_call
375: // output item for round-tripping through multi-turn conversations.
376: // The ItemID is used with item_reference for efficient round-tripping
377: // when response storage is enabled.
378: type WebSearchCallMetadata struct {
379: 	// ItemID is the server-side ID of the web_search_call output item.
380: 	ItemID string `json:"item_id"`
381: 	// Action contains the structured action data from the search.
382: 	Action *WebSearchAction `json:"action,omitempty"`
383: }
384: 
385: // Options implements the ProviderOptionsData interface.
386: func (*WebSearchCallMetadata) Options() {}
387: 
388: // MarshalJSON implements custom JSON marshaling with type info.
389: func (m WebSearchCallMetadata) MarshalJSON() ([]byte, error) {
390: 	type plain WebSearchCallMetadata
391: 	return fantasy.MarshalProviderType(TypeWebSearchCallMetadata, plain(m))
392: }
393: 
394: // UnmarshalJSON implements custom JSON unmarshaling with type info.
395: func (m *WebSearchCallMetadata) UnmarshalJSON(data []byte) error {
396: 	type plain WebSearchCallMetadata
397: 	var p plain
398: 	if err := fantasy.UnmarshalProviderType(data, &p); err != nil {
399: 		return err
400: 	}
401: 	*m = WebSearchCallMetadata(p)
402: 	return nil
403: }
```
