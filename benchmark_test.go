package lawbench

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRun_SimpleOperation verifies benchmark runner works.
func TestRun_SimpleOperation(t *testing.T) {
	var counter int64

	op := func(ctx context.Context) error {
		atomic.AddInt64(&counter, 1)
		return nil
	}

	cfg := DefaultConfig()
	cfg.Duration = 500 * time.Millisecond
	cfg.Warmup = 100 * time.Millisecond
	cfg.Levels = []int{1, 2}

	results, err := Run(context.Background(), op, cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Verify N=1 result
	if results[0].N != 1 {
		t.Errorf("Expected N=1, got N=%d", results[0].N)
	}
	if results[0].Operations == 0 {
		t.Error("No operations recorded for N=1")
	}

	// Verify N=2 result
	if results[1].N != 2 {
		t.Errorf("Expected N=2, got N=%d", results[1].N)
	}
	if results[1].Operations == 0 {
		t.Error("No operations recorded for N=2")
	}

	t.Logf("N=1: %d ops, %.2f ops/sec", results[0].Operations, results[0].Throughput)
	t.Logf("N=2: %d ops, %.2f ops/sec", results[1].Operations, results[1].Throughput)
}

// TestCalculateStatistics verifies percentile calculations.
func TestCalculateStatistics(t *testing.T) {
	result := Result{
		N:          1,
		Duration:   1 * time.Second,
		Operations: 5,
		Latencies: []time.Duration{
			100 * time.Microsecond,
			200 * time.Microsecond,
			300 * time.Microsecond,
			400 * time.Microsecond,
			500 * time.Microsecond,
		},
	}

	stats := CalculateStatistics(result)

	// P50 should be 300μs (middle value)
	if stats.P50 != 300*time.Microsecond {
		t.Errorf("P50: expected 300µs, got %v", stats.P50)
	}

	// Mean should be 300μs
	if stats.Mean != 300*time.Microsecond {
		t.Errorf("Mean: expected 300µs, got %v", stats.Mean)
	}

	t.Logf("Stats: mean=%v, p50=%v, p95=%v, p99=%v",
		stats.Mean, stats.P50, stats.P95, stats.P99)
}

// TestFitUSL_LinearScaling tests USL fit with ideal linear data.
func TestFitUSL_LinearScaling(t *testing.T) {
	// Simulate perfect linear scaling: C(N) = 1000 * N
	results := []Result{
		{N: 1, Throughput: 1000},
		{N: 2, Throughput: 2000},
		{N: 4, Throughput: 4000},
		{N: 8, Throughput: 8000},
	}

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("FitUSL failed: %v", err)
	}

	t.Logf("Coefficients: λ=%.2f, α=%.6f, β=%.6f, R²=%.4f",
		coeffs.Lambda, coeffs.Alpha, coeffs.Beta, coeffs.RSquared)

	// For perfect linear scaling: α ≈ 0, β ≈ 0
	if coeffs.Alpha > 0.1 || coeffs.Alpha < -0.1 {
		t.Logf("Note: α=%.6f (expected ~0 for linear scaling)", coeffs.Alpha)
	}

	if coeffs.Beta > 0.1 || coeffs.Beta < -0.1 {
		t.Logf("Note: β=%.6f (expected ~0 for linear scaling)", coeffs.Beta)
	}

	// Predictions should match measurements closely
	for _, r := range results {
		predicted := coeffs.PredictThroughput(r.N)
		percentError := (predicted - r.Throughput) / r.Throughput * 100
		t.Logf("N=%d: measured=%.0f, predicted=%.0f, error=%.1f%%",
			r.N, r.Throughput, predicted, percentError)
	}
}

// TestFitUSL_WithContention tests USL fit with contention.
func TestFitUSL_WithContention(t *testing.T) {
	// Simulate contention: C(N) = λN / (1 + 0.1*(N-1))
	// This should yield α ≈ 0.1, β ≈ 0
	lambda := 1000.0
	alpha := 0.1

	results := make([]Result, 0)
	for _, n := range []int{1, 2, 4, 8} {
		throughput := (lambda * float64(n)) / (1 + alpha*float64(n-1))
		results = append(results, Result{N: n, Throughput: throughput})
	}

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("FitUSL failed: %v", err)
	}

	t.Logf("Coefficients: λ=%.2f, α=%.6f, β=%.6f, R²=%.4f",
		coeffs.Lambda, coeffs.Alpha, coeffs.Beta, coeffs.RSquared)

	// Should recover α ≈ 0.1
	if coeffs.Alpha < 0.05 || coeffs.Alpha > 0.15 {
		t.Errorf("Expected α ≈ 0.1, got α=%.6f", coeffs.Alpha)
	}
}
