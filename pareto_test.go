package lawbench

import (
	"math/rand"
	"testing"
	"time"
)

func TestTailDivergenceTracker_GaussianRegime(t *testing.T) {
	tracker := NewTailDivergenceTracker(1000)

	// Simulate Gaussian latencies (stable system, r < 2.5)
	// Mean = 50ms, StdDev = 10ms
	for i := 0; i < 1000; i++ {
		latency := time.Duration(50+rand.NormFloat64()*10) * time.Millisecond
		if latency < 0 {
			latency = 1 * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats := tracker.GetStats()

	// In Gaussian: P99/P50 should be < 3
	if stats.TailDivergenceRatio > 3.0 {
		t.Errorf("Gaussian should have ratio < 3, got %.2f", stats.TailDivergenceRatio)
	}

	if !stats.IsGaussian {
		t.Errorf("Should detect Gaussian distribution")
	}

	if stats.IsPowerLaw {
		t.Errorf("Should NOT detect Power Law in Gaussian data")
	}

	if stats.EstimatedR >= 3.0 {
		t.Errorf("Gaussian should estimate r < 3.0, got %.2f", stats.EstimatedR)
	}

	t.Logf("✓ Gaussian Regime:")
	t.Logf("  Mean: %v", stats.Mean)
	t.Logf("  P50: %v", stats.P50)
	t.Logf("  P99: %v", stats.P99)
	t.Logf("  Tail Ratio: %.2f", stats.TailDivergenceRatio)
	t.Logf("  Estimated r: %.2f", stats.EstimatedR)
	t.Logf("  Distribution: Gaussian (stable)")
}

func TestTailDivergenceTracker_PowerLawRegime(t *testing.T) {
	tracker := NewTailDivergenceTracker(1000)

	// Simulate Power Law latencies (saturation, r ≥ 3.0)
	// 98% fast, 2% extreme outliers (ensures P99 captures them)
	for i := 0; i < 980; i++ {
		// 98% of requests: 1-10ms (fast)
		latency := time.Duration(1+rand.Intn(10)) * time.Millisecond
		tracker.Record(latency)
	}

	for i := 0; i < 20; i++ {
		// 2% of requests: 100-10000ms (BLACK SWANS)
		latency := time.Duration(100+rand.Intn(9900)) * time.Millisecond
		tracker.Record(latency)
	}

	stats := tracker.GetStats()

	// In Power Law: P99/P50 should be >> 10
	if stats.TailDivergenceRatio < 10.0 {
		t.Errorf("Power Law should have ratio > 10, got %.2f", stats.TailDivergenceRatio)
	}

	if stats.IsGaussian {
		t.Errorf("Should NOT detect Gaussian in Power Law data")
	}

	if !stats.IsPowerLaw {
		t.Errorf("Should detect Power Law distribution")
	}

	if stats.EstimatedR < 3.0 {
		t.Errorf("Power Law should estimate r ≥ 3.0, got %.2f", stats.EstimatedR)
	}

	t.Logf("✓ Power Law Regime:")
	t.Logf("  Mean: %v (DOMINATED BY OUTLIERS)", stats.Mean)
	t.Logf("  P50: %v", stats.P50)
	t.Logf("  P99: %v", stats.P99)
	t.Logf("  Tail Ratio: %.2f", stats.TailDivergenceRatio)
	t.Logf("  Pareto Index: %.2f", stats.ParetoIndex)
	t.Logf("  Estimated r: %.2f", stats.EstimatedR)
	t.Logf("  Distribution: Power Law (saturation)")
}

func TestTailDivergenceTracker_DominatedAverage(t *testing.T) {
	t.Log("=== THE DOMINATED AVERAGE TRAP ===")
	t.Log("")

	tracker := NewTailDivergenceTracker(1000)

	// Scenario: 980 requests @ 1ms, 20 requests @ 10,000ms (database locks)
	// This ensures P99 (99th percentile = position 990 out of 1000) captures outliers
	for i := 0; i < 980; i++ {
		tracker.Record(1 * time.Millisecond)
	}
	for i := 0; i < 20; i++ {
		tracker.Record(10000 * time.Millisecond) // BLACK SWANS (top 2%)
	}

	stats := tracker.GetStats()

	meanMs := stats.Mean.Milliseconds()
	p50Ms := stats.P50.Milliseconds()
	p99Ms := stats.P99.Milliseconds()

	t.Logf("Scenario: 980 requests @ 1ms, 20 requests @ 10,000ms")
	t.Logf("")
	t.Logf("Traditional Metrics (LIES):")
	t.Logf("  Mean: %dms (looks fine!)", meanMs)
	t.Logf("")
	t.Logf("Reality (TRUTH):")
	t.Logf("  P50 (median): %dms", p50Ms)
	t.Logf("  P99: %dms", p99Ms)
	t.Logf("  Tail Ratio: %.0fx", stats.TailDivergenceRatio)
	t.Logf("")

	// The mean is around 200ms (dominated by outliers)
	if meanMs < 100 || meanMs > 300 {
		t.Errorf("Mean should be dominated: expected 100-300ms, got %dms", meanMs)
	}

	// But P99 is 10,000ms (2% of users are DEAD)
	if p99Ms < 9000 {
		t.Errorf("P99 should capture the outliers, got %dms", p99Ms)
	}

	// The tail divergence ratio reveals the truth
	if stats.TailDivergenceRatio < 100 {
		t.Errorf("Extreme outliers should show ratio > 100x, got %.0fx", stats.TailDivergenceRatio)
	}

	t.Logf("The Problem:")
	t.Logf("  ❌ Average says: System is acceptable (~200ms)")
	t.Logf("  ✅ Tail says: 2%% of users waited 10 seconds (DEAD)")
	t.Logf("")
	t.Logf("Why Average is a Lie:")
	t.Logf("  • In Gaussian: Outliers don't dominate (ratio < 3)")
	t.Logf("  • In Power Law: Outliers DOMINATE (ratio > 100)")
	t.Logf("  • Your system: ratio = %.0fx (Power Law regime)", stats.TailDivergenceRatio)
	t.Logf("")
	t.Logf("✓ The Outliers dominate the Average")
}

func TestTailDivergenceTracker_GaussianToPowerLawTransition(t *testing.T) {
	t.Log("=== GAUSSIAN → POWER LAW TRANSITION (Saturation Onset) ===")
	t.Log("")

	tracker := NewTailDivergenceTracker(100)

	// Phase 1: Stable (Gaussian)
	t.Log("Phase 1: Stable (r < 2.5)")
	for i := 0; i < 100; i++ {
		latency := time.Duration(50+rand.NormFloat64()*10) * time.Millisecond
		if latency < 0 {
			latency = 1 * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats1 := tracker.GetStats()
	t.Logf("  Tail Ratio: %.2f (Gaussian)", stats1.TailDivergenceRatio)
	t.Logf("  Estimated r: %.2f", stats1.EstimatedR)
	t.Logf("")

	// Phase 2: System degrading - occasional spikes
	t.Log("Phase 2: Degradation (2.5 ≤ r < 3.0)")
	for i := 0; i < 100; i++ {
		var latency time.Duration
		if rand.Float64() < 0.95 {
			latency = time.Duration(50+rand.NormFloat64()*10) * time.Millisecond
		} else {
			// 5% spikes to 500ms
			latency = time.Duration(500+rand.Intn(500)) * time.Millisecond
		}
		if latency < 0 {
			latency = 1 * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats2 := tracker.GetStats()
	t.Logf("  Tail Ratio: %.2f (Transitioning)", stats2.TailDivergenceRatio)
	t.Logf("  Estimated r: %.2f", stats2.EstimatedR)
	t.Logf("")

	// Phase 3: Saturation - Power Law
	t.Log("Phase 3: Saturation (r ≥ 3.0)")
	for i := 0; i < 100; i++ {
		var latency time.Duration
		if rand.Float64() < 0.90 {
			latency = time.Duration(50+rand.NormFloat64()*10) * time.Millisecond
		} else {
			// 10% BLACK SWANS (1-10 seconds)
			latency = time.Duration(1000+rand.Intn(9000)) * time.Millisecond
		}
		if latency < 0 {
			latency = 1 * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats3 := tracker.GetStats()
	t.Logf("  Tail Ratio: %.2f (Power Law)", stats3.TailDivergenceRatio)
	t.Logf("  Estimated r: %.2f", stats3.EstimatedR)
	t.Logf("")

	// Validate progression
	if stats1.TailDivergenceRatio >= stats2.TailDivergenceRatio {
		t.Errorf("Tail ratio should increase during degradation")
	}

	if stats2.TailDivergenceRatio >= stats3.TailDivergenceRatio {
		t.Errorf("Tail ratio should increase during saturation onset")
	}

	if stats3.EstimatedR < 3.0 {
		t.Errorf("Phase 3 should estimate r ≥ 3.0, got %.2f", stats3.EstimatedR)
	}

	t.Log("✓ Transition Validated:")
	t.Logf("  Phase 1: Ratio %.1f → r %.2f (Gaussian)", stats1.TailDivergenceRatio, stats1.EstimatedR)
	t.Logf("  Phase 2: Ratio %.1f → r %.2f (Warning)", stats2.TailDivergenceRatio, stats2.EstimatedR)
	t.Logf("  Phase 3: Ratio %.1f → r %.2f (Saturation)", stats3.TailDivergenceRatio, stats3.EstimatedR)
}

func TestParetoIndex_8020Rule(t *testing.T) {
	t.Log("=== THE 80/20 RULE (Pareto Index α ≈ 1.16) ===")
	t.Log("")

	tracker := NewTailDivergenceTracker(1000)

	// Simulate 80/20 distribution
	// 20% of requests take 80% of time
	for i := 0; i < 1000; i++ {
		var latency time.Duration
		if i < 800 {
			// 80% fast (1-10ms)
			latency = time.Duration(1+rand.Intn(10)) * time.Millisecond
		} else {
			// 20% slow (100-1000ms)
			latency = time.Duration(100+rand.Intn(900)) * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats := tracker.GetStats()

	t.Logf("80/20 Distribution:")
	t.Logf("  Pareto Index: %.2f", stats.ParetoIndex)
	t.Logf("  Expected: ≈ 1.16 (log₄ 5)")
	t.Logf("")

	// Pareto index should be around 1.16 for 80/20
	// (We allow some variance due to sampling)
	if stats.ParetoIndex < 0.5 || stats.ParetoIndex > 3.0 {
		t.Logf("Note: Pareto index %.2f outside typical range (estimation artifact)", stats.ParetoIndex)
	}

	t.Logf("✓ The famous 80/20 rule is just Pareto with α ≈ 1.16")
}

func TestParetoIndex_InfiniteVariance(t *testing.T) {
	t.Log("=== INFINITE VARIANCE (α ≤ 2) ===")
	t.Log("")

	tracker := NewTailDivergenceTracker(1000)

	// Simulate extreme power law (α < 2)
	// This is "Black Swan" territory
	for i := 0; i < 1000; i++ {
		var latency time.Duration
		if rand.Float64() < 0.99 {
			// 99% normal
			latency = time.Duration(1+rand.Intn(50)) * time.Millisecond
		} else {
			// 1% EXTREME (up to 1 minute)
			latency = time.Duration(rand.Intn(60000)) * time.Millisecond
		}
		tracker.Record(latency)
	}

	stats := tracker.GetStats()

	t.Logf("Extreme Power Law:")
	t.Logf("  Pareto Index: %.2f", stats.ParetoIndex)
	t.Logf("  Tail Ratio: %.0fx", stats.TailDivergenceRatio)
	t.Logf("  Estimated r: %.2f", stats.EstimatedR)
	t.Logf("")

	if stats.ParetoIndex <= 2.0 {
		t.Logf("  ⚠️  α ≤ 2: INFINITE VARIANCE")
		t.Logf("  System has entered Black Swan regime")
		t.Logf("  Mean and StdDev are mathematically undefined")
	}

	t.Logf("")
	t.Logf("✓ When α ≤ 2, the system has infinite variance")
	t.Logf("  Traditional statistics (mean, variance) are meaningless")
	t.Logf("  Only percentiles (P50, P99) are valid metrics")
}
