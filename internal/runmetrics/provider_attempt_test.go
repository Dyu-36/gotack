package runmetrics

import (
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/stretchr/testify/require"
)

func TestValidateProviderAttemptTelemetry(t *testing.T) {
	first := int64(10)
	frame := int64(45)
	delta := int64(35)
	base := func() *crushapi.RunTelemetry {
		return &crushapi.RunTelemetry{CacheStatus: "unreported", ProviderAttempts: []crushapi.ProviderAttemptTelemetry{{
			ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop",
			FirstResponseByteMicros: &first, FirstSSEFrameMicros: &frame, FirstByteToFirstSSEMicros: &delta,
		}}}
	}
	require.NoError(t, Validate(base()))

	badIdentity := base()
	badIdentity.ProviderAttempts[0].ModelCallID = 0
	require.Error(t, Validate(badIdentity))

	badPurpose := base()
	badPurpose.ProviderAttempts[0].Purpose = "prep_error"
	require.Error(t, Validate(badPurpose))

	negative := int64(-1)
	badTiming := base()
	badTiming.ProviderAttempts[0].RequestWrittenMicros = &negative
	require.Error(t, Validate(badTiming))

	wrongDelta := int64(34)
	badSpan := base()
	badSpan.ProviderAttempts[0].FirstByteToFirstSSEMicros = &wrongDelta
	require.Error(t, Validate(badSpan))
}

func TestRedactionDeepCopiesProviderAttempts(t *testing.T) {
	delta := int64(35)
	original := &crushapi.RunTelemetry{CacheStatus: "unreported", ProviderAttempts: []crushapi.ProviderAttemptTelemetry{{
		ModelCallID: 1, HTTPAttempt: 1, Purpose: "tool_loop", FirstByteToFirstSSEMicros: &delta,
	}}}
	copy := redactSensitive(original)
	copy.ProviderAttempts[0].Purpose = "retry"
	*copy.ProviderAttempts[0].FirstByteToFirstSSEMicros = 99
	require.Equal(t, "tool_loop", original.ProviderAttempts[0].Purpose)
	require.Equal(t, int64(35), *original.ProviderAttempts[0].FirstByteToFirstSSEMicros)
}
