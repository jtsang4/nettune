// Package probe provides network testing functionality
package probe

import (
	"sort"
	"sync"
	"time"

	"github.com/jtsang4/nettune/internal/client/http"
	"github.com/jtsang4/nettune/internal/shared/types"
)

// RTTTester performs RTT measurements
type RTTTester struct {
	client *http.Client
}

// NewRTTTester creates a new RTT tester
func NewRTTTester(client *http.Client) *RTTTester {
	return &RTTTester{client: client}
}

// TestRTT performs RTT measurements
func (t *RTTTester) TestRTT(count, concurrency int) (*types.RTTResult, error) {
	if count <= 0 {
		count = 30
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	results := make(chan float64, count)
	errors := make(chan string, count)

	// Semaphore for concurrency control
	sem := make(chan struct{}, concurrency)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			_, err := t.client.ProbeEcho()
			duration := time.Since(start)

			if err != nil {
				errors <- err.Error()
				return
			}
			results <- float64(duration.Milliseconds())
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Collect results
	var rtts []float64
	for rtt := range results {
		rtts = append(rtts, rtt)
	}

	var errs []string
	for err := range errors {
		errs = append(errs, err)
	}

	result := &types.RTTResult{
		Count:      count,
		Successful: len(rtts),
		Failed:     len(errs),
		Errors:     errs,
	}

	if len(rtts) > 0 {
		result.RTT = calculateStats(rtts)
		result.Jitter = calculateJitter(rtts)
	}

	return result, nil
}

// calculateStats calculates latency statistics
func calculateStats(values []float64) *types.LatencyStats {
	if len(values) == 0 {
		return nil
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	stats := &types.LatencyStats{
		Min:  sorted[0],
		Max:  sorted[len(sorted)-1],
		Mean: mean(sorted),
		P50:  percentile(sorted, 50),
		P90:  percentile(sorted, 90),
		P99:  percentile(sorted, 99),
	}

	return stats
}

// calculateJitter calculates jitter (average deviation from mean)
func calculateJitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	m := mean(values)
	var sumDiff float64
	for _, v := range values {
		diff := v - m
		if diff < 0 {
			diff = -diff
		}
		sumDiff += diff
	}
	return sumDiff / float64(len(values))
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}
