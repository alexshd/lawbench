package lawbench

import (
	"fmt"
	"testing"
)

// AssertionConfig contains thresholds for scalability properties.
type AssertionConfig struct {
	// Contention threshold (α < this value passes)
	MaxContention float64

	// Coordination threshold (β < this value passes)
	MaxCoordination float64

	// Minimum R² for model fit quality
	MinRSquared float64

	// Linear scaling tolerance (1.0 = perfect)
	MinEfficiency float64

	// Maximum concurrency to test retrograde behavior
	MaxN int
}

// DefaultAssertionConfig returns conservative thresholds.
func DefaultAssertionConfig() AssertionConfig {
	return AssertionConfig{
		MaxContention:   0.01, // 1% contention
		MaxCoordination: 0.01, // 1% coordination overhead
		MinRSquared:     0.95, // 95% model fit
		MinEfficiency:   0.95, // 95% of ideal throughput
		MaxN:            16,   // Test up to 16 cores
	}
}

// AssertZeroContention verifies α (contention coefficient) is near zero.
//
// Zero contention means the system is lock-free or uses efficient
// synchronization primitives. α < 0.01 indicates excellent concurrency.
//
// Mathematical property:
//
//	∂C/∂N ≈ λ when α ≈ 0 (throughput grows linearly)
func AssertZeroContention(t *testing.T, results []Result, cfg AssertionConfig) {
	t.Helper()

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("Failed to fit USL model: %v", err)
	}

	if coeffs.Alpha > cfg.MaxContention {
		t.Errorf("Contention too high: α = %.6f (max: %.6f)\n"+
			"System shows lock contention. Consider lock-free data structures.",
			coeffs.Alpha, cfg.MaxContention)
	}

	if coeffs.RSquared < cfg.MinRSquared {
		t.Errorf("Poor model fit: R² = %.4f (min: %.4f)\n"+
			"USL model doesn't explain the data. Check for measurement noise.",
			coeffs.RSquared, cfg.MinRSquared)
	}

	t.Logf("✓ Zero contention: α = %.6f (threshold: %.6f)", coeffs.Alpha, cfg.MaxContention)
	t.Logf("  Model fit: R² = %.4f", coeffs.RSquared)
}

// AssertZeroCoordination verifies β (coordination coefficient) is near zero.
//
// Zero coordination means no cache coherency traffic or inter-core
// communication overhead. β < 0.01 indicates excellent cache locality.
//
// Mathematical property:
//
//	C(N) ≈ λN when β ≈ 0 (no quadratic slowdown)
//
// Note: β < 0 indicates superlinear scaling (cache-friendly batching).
func AssertZeroCoordination(t *testing.T, results []Result, cfg AssertionConfig) {
	t.Helper()

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("Failed to fit USL model: %v", err)
	}

	if coeffs.Beta > cfg.MaxCoordination {
		t.Errorf("Coordination overhead too high: β = %.6f (max: %.6f)\n"+
			"System shows cache coherency or communication overhead.",
			coeffs.Beta, cfg.MaxCoordination)
	}

	if coeffs.Beta < 0 {
		t.Logf("✓ Superlinear scaling: β = %.6f (negative indicates cache-friendliness)", coeffs.Beta)
	} else {
		t.Logf("✓ Zero coordination: β = %.6f (threshold: %.6f)", coeffs.Beta, cfg.MaxCoordination)
	}

	t.Logf("  Model fit: R² = %.4f", coeffs.RSquared)
}

// AssertLinearScaling verifies throughput scales proportionally with cores.
//
// Linear scaling means C(N) ≈ λN (efficiency ≈ 1.0 at all N).
// This is the gold standard for parallel systems.
//
// Mathematical property:
//
//	C(N) / (λN) > 0.95 for all N ≤ MaxN
func AssertLinearScaling(t *testing.T, results []Result, cfg AssertionConfig) {
	t.Helper()

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("Failed to fit USL model: %v", err)
	}

	var failures []string
	for _, r := range results {
		if r.N > cfg.MaxN {
			continue
		}

		efficiency := coeffs.Efficiency(r.N)
		if efficiency < cfg.MinEfficiency {
			failures = append(failures, fmt.Sprintf(
				"  N=%d: efficiency=%.2f%% (min: %.2f%%)",
				r.N, efficiency*100, cfg.MinEfficiency*100))
		}
	}

	if len(failures) > 0 {
		t.Errorf("Scaling not linear:\n%s\nα=%.6f, β=%.6f",
			failures, coeffs.Alpha, coeffs.Beta)
	}

	t.Logf("✓ Linear scaling: efficiency > %.1f%% for N ≤ %d", cfg.MinEfficiency*100, cfg.MaxN)
	t.Logf("  α=%.6f, β=%.6f, R²=%.4f", coeffs.Alpha, coeffs.Beta, coeffs.RSquared)
}

// AssertNoRetrograde verifies throughput never decreases as N increases.
//
// Retrograde scaling means C(N+1) < C(N) - throughput decreases with
// more workers. This indicates severe contention or coordination overhead.
//
// Mathematical property:
//
//	∂C/∂N > 0 for all N ≤ MaxN
func AssertNoRetrograde(t *testing.T, results []Result, cfg AssertionConfig) {
	t.Helper()

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("Failed to fit USL model: %v", err)
	}

	var failures []string
	for i := 1; i < len(results); i++ {
		if results[i].N > cfg.MaxN {
			break
		}

		prevThroughput := coeffs.PredictThroughput(results[i-1].N)
		currThroughput := coeffs.PredictThroughput(results[i].N)

		if currThroughput < prevThroughput {
			failures = append(failures, fmt.Sprintf(
				"  N=%d→%d: %.2f → %.2f ops/sec (retrograde!)",
				results[i-1].N, results[i].N, prevThroughput, currThroughput))
		}
	}

	if len(failures) > 0 {
		t.Errorf("Retrograde scaling detected:\n%s\nα=%.6f, β=%.6f",
			failures, coeffs.Alpha, coeffs.Beta)
	}

	t.Logf("✓ No retrograde: throughput increases monotonically up to N=%d", cfg.MaxN)
	t.Logf("  α=%.6f, β=%.6f, R²=%.4f", coeffs.Alpha, coeffs.Beta, coeffs.RSquared)
}

// AssertScalability runs all scalability assertions with default config.
func AssertScalability(t *testing.T, results []Result) {
	t.Helper()

	cfg := DefaultAssertionConfig()

	t.Run("ZeroContention", func(t *testing.T) {
		AssertZeroContention(t, results, cfg)
	})

	t.Run("ZeroCoordination", func(t *testing.T) {
		AssertZeroCoordination(t, results, cfg)
	})

	t.Run("LinearScaling", func(t *testing.T) {
		AssertLinearScaling(t, results, cfg)
	})

	t.Run("NoRetrograde", func(t *testing.T) {
		AssertNoRetrograde(t, results, cfg)
	})
}

// PrintAnalysis outputs detailed USL analysis to the test log.
func PrintAnalysis(t *testing.T, results []Result) {
	t.Helper()

	coeffs, err := FitUSL(results)
	if err != nil {
		t.Fatalf("Failed to fit USL model: %v", err)
	}

	t.Logf("\n=== USL Analysis ===")
	t.Logf("Coefficients:")
	t.Logf("  λ (lambda)  = %.2f ops/sec (serial performance)", coeffs.Lambda)
	t.Logf("  α (alpha)   = %.6f (contention)", coeffs.Alpha)
	t.Logf("  β (beta)    = %.6f (coordination)", coeffs.Beta)
	t.Logf("  R²          = %.4f (goodness of fit)", coeffs.RSquared)

	t.Logf("\nMeasured vs Predicted:")
	t.Logf("  N    Measured      Predicted     Efficiency")
	t.Logf("  --   ------------  ------------  ----------")
	for _, r := range results {
		predicted := coeffs.PredictThroughput(r.N)
		efficiency := coeffs.Efficiency(r.N)
		t.Logf("  %-4d %12.2f  %12.2f  %8.1f%%",
			r.N, r.Throughput, predicted, efficiency*100)
	}

	t.Logf("\nCapacity Planning:")
	for _, n := range []int{32, 64, 128} {
		predicted := coeffs.PredictThroughput(n)
		efficiency := coeffs.Efficiency(n)
		t.Logf("  N=%-3d: %12.2f ops/sec (efficiency: %.1f%%)",
			n, predicted, efficiency*100)
	}

	// Interpret coefficients
	t.Logf("\nInterpretation:")
	if coeffs.Alpha < 0.01 {
		t.Logf("  ✓ Excellent contention (α < 0.01) - lock-free or efficient locks")
	} else if coeffs.Alpha < 0.05 {
		t.Logf("  ⚠ Moderate contention (α < 0.05) - some lock waiting")
	} else {
		t.Logf("  ✗ High contention (α ≥ 0.05) - significant lock bottleneck")
	}

	if coeffs.Beta < 0 {
		t.Logf("  ✓ Superlinear scaling (β < 0) - cache-friendly workload")
	} else if coeffs.Beta < 0.01 {
		t.Logf("  ✓ Excellent coordination (β < 0.01) - minimal cache coherency")
	} else if coeffs.Beta < 0.05 {
		t.Logf("  ⚠ Moderate coordination (β < 0.05) - some communication overhead")
	} else {
		t.Logf("  ✗ High coordination (β ≥ 0.05) - severe cache/communication bottleneck")
	}

	if coeffs.RSquared > 0.98 {
		t.Logf("  ✓ Excellent model fit (R² > 0.98)")
	} else if coeffs.RSquared > 0.95 {
		t.Logf("  ✓ Good model fit (R² > 0.95)")
	} else if coeffs.RSquared > 0.90 {
		t.Logf("  ⚠ Fair model fit (R² > 0.90)")
	} else {
		t.Logf("  ✗ Poor model fit (R² < 0.90) - check for measurement noise")
	}
}
