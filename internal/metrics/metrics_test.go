package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordRequest(t *testing.T) {
	// Record a request and verify counter increases
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/test", "200"))
	RecordRequest("GET", "/test", 200)
	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/test", "200"))

	assert.Equal(t, before+1, after)
}

func TestRecordDuration(t *testing.T) {
	// Record duration - just verify it doesn't panic
	RecordDuration("POST", "/run", 0.5)
	RecordDuration("POST", "/run", 1.5)

	// Verify collector has metrics
	count := testutil.CollectAndCount(httpDuration)
	assert.Greater(t, count, 0)
}

func TestActiveContainersGauge(t *testing.T) {
	SetActiveContainers(5)
	assert.Equal(t, float64(5), testutil.ToFloat64(activeContainers))

	IncActiveContainers()
	assert.Equal(t, float64(6), testutil.ToFloat64(activeContainers))

	DecActiveContainers()
	assert.Equal(t, float64(5), testutil.ToFloat64(activeContainers))

	SetActiveContainers(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(activeContainers))
}
