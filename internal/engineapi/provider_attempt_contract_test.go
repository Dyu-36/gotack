package engineapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunTelemetryDecodesProviderAttemptIdentity(t *testing.T) {
	var got RunTelemetry
	err := json.Unmarshal([]byte(`{"run_id":"run-1","provider_attempts":[{"model_call_id":2,"http_attempt":1,"purpose":"tool_loop","first_response_byte_us":10,"first_sse_frame_us":45,"first_byte_to_first_sse_us":35}]}`), &got)
	require.NoError(t, err)
	require.Len(t, got.ProviderAttempts, 1)
	a := got.ProviderAttempts[0]
	require.Equal(t, 2, a.ModelCallID)
	require.Equal(t, 1, a.HTTPAttempt)
	require.Equal(t, "tool_loop", a.Purpose)
	require.NotNil(t, a.FirstByteToFirstSSEMicros)
	require.Equal(t, int64(35), *a.FirstByteToFirstSSEMicros)
}
