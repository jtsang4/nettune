package probe

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/jtsang4/nettune/internal/client/http"
	"github.com/jtsang4/nettune/internal/shared/types"
)

// ThroughputTester performs throughput measurements
type ThroughputTester struct {
	client *http.Client
}

// NewThroughputTester creates a new throughput tester
func NewThroughputTester(client *http.Client) *ThroughputTester {
	return &ThroughputTester{client: client}
}

// TestDownload performs download throughput test with multiple iterations
func (t *ThroughputTester) TestDownload(bytes int64, parallel int) (*types.ThroughputResult, error) {
	return t.TestDownloadWithIterations(bytes, parallel, 1)
}

// TestDownloadWithIterations performs download throughput test with specified iterations
func (t *ThroughputTester) TestDownloadWithIterations(bytes int64, parallel, iterations int) (*types.ThroughputResult, error) {
	if bytes <= 0 {
		bytes = 100 * 1024 * 1024 // 100MB default
	}
	if parallel <= 0 {
		parallel = 1
	}
	if iterations <= 0 {
		iterations = 1
	}

	var allResults []float64
	var totalBytes int64
	var totalDuration time.Duration
	var allErrors []string

	for iter := 0; iter < iterations; iter++ {
		bytesPerConnection := bytes / int64(parallel)
		var wg sync.WaitGroup

		type connResult struct {
			bytes    int64
			duration time.Duration
			err      error
		}
		results := make(chan connResult, parallel)

		start := time.Now()

		for i := 0; i < parallel; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				received, duration, err := t.client.ProbeDownload(bytesPerConnection)
				results <- connResult{bytes: received, duration: duration, err: err}
			}()
		}

		wg.Wait()
		iterDuration := time.Since(start)
		close(results)

		var iterBytes int64
		for r := range results {
			iterBytes += r.bytes
			if r.err != nil {
				allErrors = append(allErrors, r.err.Error())
			}
		}

		totalBytes += iterBytes
		totalDuration += iterDuration
		iterThroughput := float64(iterBytes*8) / float64(iterDuration.Milliseconds()) / 1000
		allResults = append(allResults, iterThroughput)
	}

	avgThroughput := mean(allResults)
	stdDev := standardDeviation(allResults, avgThroughput)

	result := &types.ThroughputResult{
		Direction:      "download",
		Bytes:          totalBytes,
		DurationMs:     totalDuration.Milliseconds(),
		ThroughputMbps: avgThroughput,
		Parallel:       parallel,
		Errors:         allErrors,
	}

	if iterations > 1 {
		result.Iterations = iterations
		result.AllResults = allResults
		result.StdDev = stdDev
	}

	return result, nil
}

// TestUpload performs upload throughput test with multiple iterations
func (t *ThroughputTester) TestUpload(bytes int64, parallel int) (*types.ThroughputResult, error) {
	return t.TestUploadWithIterations(bytes, parallel, 1)
}

// TestUploadWithIterations performs upload throughput test with specified iterations
func (t *ThroughputTester) TestUploadWithIterations(bytes int64, parallel, iterations int) (*types.ThroughputResult, error) {
	if bytes <= 0 {
		bytes = 100 * 1024 * 1024 // 100MB default
	}
	if parallel <= 0 {
		parallel = 1
	}
	if iterations <= 0 {
		iterations = 1
	}

	bytesPerConnection := bytes / int64(parallel)
	// Generate random data once, reuse across iterations
	data := make([]byte, bytesPerConnection)
	rand.Read(data)

	var allResults []float64
	var totalBytes int64
	var totalDuration time.Duration
	var allErrors []string

	for iter := 0; iter < iterations; iter++ {
		var wg sync.WaitGroup

		type connResult struct {
			bytes    int64
			duration time.Duration
			err      error
		}
		results := make(chan connResult, parallel)

		start := time.Now()

		for i := 0; i < parallel; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				uploadStart := time.Now()
				resp, err := t.client.ProbeUpload(data)
				duration := time.Since(uploadStart)

				if err != nil {
					results <- connResult{err: err, duration: duration}
					return
				}
				results <- connResult{bytes: resp.ReceivedBytes, duration: duration}
			}()
		}

		wg.Wait()
		iterDuration := time.Since(start)
		close(results)

		var iterBytes int64
		for r := range results {
			iterBytes += r.bytes
			if r.err != nil {
				allErrors = append(allErrors, r.err.Error())
			}
		}

		totalBytes += iterBytes
		totalDuration += iterDuration
		iterThroughput := float64(iterBytes*8) / float64(iterDuration.Milliseconds()) / 1000
		allResults = append(allResults, iterThroughput)
	}

	avgThroughput := mean(allResults)
	stdDev := standardDeviation(allResults, avgThroughput)

	result := &types.ThroughputResult{
		Direction:      "upload",
		Bytes:          totalBytes,
		DurationMs:     totalDuration.Milliseconds(),
		ThroughputMbps: avgThroughput,
		Parallel:       parallel,
		Errors:         allErrors,
	}

	if iterations > 1 {
		result.Iterations = iterations
		result.AllResults = allResults
		result.StdDev = stdDev
	}

	return result, nil
}
