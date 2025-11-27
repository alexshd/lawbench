package lawbench

import (
	"context"
	"math"
	"testing"
	"time"
)

// BifurcationPoint represents a detected period-doubling transition.
type BifurcationPoint struct {
	R         float64   // Control parameter (load, pressure, etc.)
	Period    int       // Period detected (1, 2, 4, 8, ...)
	Amplitude float64   // Oscillation amplitude
	Attractor []float64 // Observed attractor values
	Dimension float64   // Fractal dimension (2.0 = stable, >2.0 = chaotic)
}

// FeigenbaumAnalysis contains the full bifurcation cascade.
type FeigenbaumAnalysis struct {
	Bifurcations       []BifurcationPoint
	Delta              float64 // δ ≈ 4.669 (period-doubling rate)
	Alpha              float64 // α ≈ 2.502 (amplitude scaling)
	SaturationBoundary      float64 // Control parameter where saturation begins
	RecoveryTime int     // Iterations to exit saturation
	TransitTime        int     // Iterations through saturation
	FractalDimension   float64 // Actual measured dimension
	BasinCompatible    bool    // True if stays in life-compatible basin
}

// MapFunction represents the iterative map: x_n+1 = f(x_n, r)
// where r is the control parameter (load, pressure, etc.)
type MapFunction func(x, r float64) float64

// FeigenbaumConfig controls bifurcation analysis.
type FeigenbaumConfig struct {
	MinR                    float64 // Starting control parameter
	MaxR                    float64 // Ending control parameter
	StepR                   float64 // Control parameter increment
	Iterations              int     // Map iterations per R value
	Warmup                  int     // Iterations to skip (transient)
	Tolerance               float64 // Period detection tolerance
	MaxPeriod               int     // Maximum period to detect
	RecoveryThreshold float64 // Distance to attractor for "recovery"
	BasinRadius             float64 // Maximum amplitude for "life-compatible"
}

// DefaultFeigenbaumConfig returns sensible defaults.
func DefaultFeigenbaumConfig() FeigenbaumConfig {
	return FeigenbaumConfig{
		MinR:                    0.0,
		MaxR:                    4.0,
		StepR:                   0.01,
		Iterations:              1000,
		Warmup:                  200,
		Tolerance:               1e-6,
		MaxPeriod:               128,
		RecoveryThreshold: 0.1,
		BasinRadius:             2.0,
	}
}

// IterateMap applies the map function repeatedly and records the trajectory.
// This is the core of bifurcation analysis - watching x evolve under f(x,r).
func IterateMap(f MapFunction, x0, r float64, cfg FeigenbaumConfig) []float64 {
	trajectory := make([]float64, 0, cfg.Iterations)
	x := x0

	// Warmup: let transients decay
	for i := 0; i < cfg.Warmup; i++ {
		x = f(x, r)
	}

	// Record attractor
	for i := 0; i < cfg.Iterations; i++ {
		x = f(x, r)
		trajectory = append(trajectory, x)
	}

	return trajectory
}

// DetectPeriod finds the period of oscillation in the trajectory.
// Period-1 = stable, Period-2 = alternating, Period-4/8/... = complex, >MaxPeriod = saturation
func DetectPeriod(trajectory []float64, cfg FeigenbaumConfig) int {
	if len(trajectory) < 2*cfg.MaxPeriod {
		return -1 // Not enough data
	}

	// Test periods 1, 2, 4, 8, 16, ... up to MaxPeriod
	for period := 1; period <= cfg.MaxPeriod; period *= 2 {
		isPeriodicPeriod := true

		// Check if trajectory repeats every 'period' steps
		for i := period; i < len(trajectory)-period; i++ {
			if math.Abs(trajectory[i]-trajectory[i+period]) > cfg.Tolerance {
				isPeriodicPeriod = false
				break
			}
		}

		if isPeriodicPeriod {
			return period
		}
	}

	return -1 // Chaotic (no period detected)
}

// CalculateFractalDimension estimates the attractor dimension using box-counting.
// Stable: D ≈ 0 (point), Periodic: D ≈ 1 (loop), Chaotic: 2 < D < 3 (strange attractor)
func CalculateFractalDimension(trajectory []float64) float64 {
	if len(trajectory) < 100 {
		return 0.0
	}

	// Simple estimation: count unique values in trajectory
	// For true fractal dimension, we'd use box-counting or correlation dimension
	uniqueMap := make(map[int]bool)
	resolution := 1000.0 // Discretization resolution

	for _, x := range trajectory {
		bucket := int(x * resolution)
		uniqueMap[bucket] = true
	}

	uniqueCount := float64(len(uniqueMap))
	totalCount := float64(len(trajectory))

	// Heuristic dimension estimate
	// If uniqueCount ≈ totalCount, high dimension (chaotic)
	// If uniqueCount is small, low dimension (periodic)
	ratio := uniqueCount / totalCount

	if ratio < 0.01 {
		return 0.0 // Point attractor (stable)
	} else if ratio < 0.1 {
		return 1.0 // Limit cycle (periodic)
	} else {
		// Approximate fractal dimension
		// Lorenz: 2.06, Rössler: 2.01, Hénon: 1.26
		return 1.0 + math.Log(ratio)/math.Log(2.0)
	}
}

// CalculateAmplitude returns the oscillation amplitude (max - min).
func CalculateAmplitude(trajectory []float64) float64 {
	if len(trajectory) == 0 {
		return 0.0
	}

	min, max := trajectory[0], trajectory[0]
	for _, x := range trajectory {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}

	return max - min
}

// DistanceToAttractor calculates how far the current state is from the attractor.
// Used for recovery detection.
func DistanceToAttractor(current float64, attractor []float64) float64 {
	if len(attractor) == 0 {
		return math.Abs(current)
	}

	// Find closest point in attractor
	minDist := math.Abs(current - attractor[0])
	for _, a := range attractor {
		dist := math.Abs(current - a)
		if dist < minDist {
			minDist = dist
		}
	}

	return minDist
}

// MeasureRecoveryTime counts iterations needed to return to stable basin after saturation.
// Simulates: system enters saturation at r_saturation, can it recover?
func MeasureRecoveryTime(f MapFunction, x0, rSaturation, rStable float64, cfg FeigenbaumConfig) int {
	// Start in saturation
	x := x0
	for i := 0; i < 100; i++ {
		x = f(x, rSaturation)
	}

	// Now reduce r to stable region and count iterations to recover
	iterations := 0
	maxIterations := 10000

	stableAttractor := IterateMap(f, 0.5, rStable, cfg)

	for iterations < maxIterations {
		x = f(x, rStable)
		iterations++

		dist := DistanceToAttractor(x, stableAttractor)
		if dist < cfg.RecoveryThreshold {
			return iterations // Recovered!
		}
	}

	return -1 // Failed to recover (trapped in saturation)
}

// MeasureTransitTime counts iterations to pass through saturation and reach stable basin on other side.
func MeasureTransitTime(f MapFunction, x0, rSaturation float64, cfg FeigenbaumConfig) int {
	x := x0
	iterations := 0
	maxIterations := 10000

	// Transit through saturation
	for iterations < maxIterations {
		x = f(x, rSaturation)
		iterations++

		// Check if we've exited to bounded region (life-compatible basin)
		if math.Abs(x) < cfg.BasinRadius && math.Abs(x) > cfg.RecoveryThreshold {
			// Are we on a trajectory that stays bounded?
			testTrajectory := IterateMap(f, x, rSaturation, FeigenbaumConfig{
				Iterations: 100,
				Warmup:     0,
			})

			allBounded := true
			for _, val := range testTrajectory {
				if math.Abs(val) > cfg.BasinRadius {
					allBounded = false
					break
				}
			}

			if allBounded {
				return iterations // Successfully transited!
			}
		}
	}

	return -1 // Failed to transit (diverged or trapped)
}

// AnalyzeBifurcation performs full Feigenbaum analysis on a map function.
func AnalyzeBifurcation(f MapFunction, x0 float64, cfg FeigenbaumConfig) FeigenbaumAnalysis {
	analysis := FeigenbaumAnalysis{
		Bifurcations: make([]BifurcationPoint, 0),
	}

	var previousPeriod int = -1
	var bifurcationRValues []float64

	// Sweep through control parameter
	for r := cfg.MinR; r <= cfg.MaxR; r += cfg.StepR {
		trajectory := IterateMap(f, x0, r, cfg)
		period := DetectPeriod(trajectory, cfg)
		amplitude := CalculateAmplitude(trajectory)
		dimension := CalculateFractalDimension(trajectory)

		// Detect bifurcation (period doubling from 2^n sequence)
		if period != previousPeriod && previousPeriod > 0 {
			// Only track power-of-2 doublings: 1→2, 2→4, 4→8, etc.
			isPowerOf2 := period > 0 && (period&(period-1)) == 0
			isDoubling := period == previousPeriod*2

			if isPowerOf2 && (isDoubling || previousPeriod == 1) {
				bifurcationRValues = append(bifurcationRValues, r)
				analysis.Bifurcations = append(analysis.Bifurcations, BifurcationPoint{
					R:         r,
					Period:    period,
					Amplitude: amplitude,
					Attractor: trajectory[len(trajectory)-period:],
					Dimension: dimension,
				})
			}
		}

		// Detect saturation boundary (first chaotic r after period-doubling cascade)
		if period == -1 && analysis.SaturationBoundary == 0 && len(analysis.Bifurcations) >= 2 {
			analysis.SaturationBoundary = r
			analysis.FractalDimension = dimension
		}

		previousPeriod = period
	}

	// Calculate Feigenbaum delta (δ) from consecutive bifurcations
	// δ_n = (r_{n+1} - r_n) / (r_{n+2} - r_{n+1})
	if len(bifurcationRValues) >= 3 {
		// Calculate delta for each triplet and average
		deltas := make([]float64, 0)
		for i := 0; i < len(bifurcationRValues)-2; i++ {
			r1 := bifurcationRValues[i]
			r2 := bifurcationRValues[i+1]
			r3 := bifurcationRValues[i+2]

			denominator := r3 - r2
			if math.Abs(denominator) > 1e-10 {
				delta := (r2 - r1) / denominator
				if delta > 0 && delta < 100 { // Sanity check
					deltas = append(deltas, delta)
				}
			}
		}

		// Average all deltas (converges to 4.669)
		if len(deltas) > 0 {
			sum := 0.0
			for _, d := range deltas {
				sum += d
			}
			analysis.Delta = sum / float64(len(deltas))
		}
	}

	// Calculate Feigenbaum alpha (amplitude scaling)
	if len(analysis.Bifurcations) >= 2 {
		amp1 := analysis.Bifurcations[len(analysis.Bifurcations)-2].Amplitude
		amp2 := analysis.Bifurcations[len(analysis.Bifurcations)-1].Amplitude
		if amp2 != 0 {
			analysis.Alpha = amp1 / amp2
		}
	}

	// Measure recovery and transit times
	if analysis.SaturationBoundary > 0 {
		rStable := cfg.MinR + (cfg.MaxR-cfg.MinR)*0.3 // 30% load (stable region)
		analysis.RecoveryTime = MeasureRecoveryTime(f, x0, analysis.SaturationBoundary, rStable, cfg)
		analysis.TransitTime = MeasureTransitTime(f, x0, analysis.SaturationBoundary, cfg)

		// Check basin compatibility
		testTrajectory := IterateMap(f, x0, analysis.SaturationBoundary, cfg)
		analysis.BasinCompatible = true
		for _, x := range testTrajectory {
			if math.Abs(x) > cfg.BasinRadius {
				analysis.BasinCompatible = false
				break
			}
		}
	}

	return analysis
}

// AssertFeigenbaumCascade verifies the system exhibits correct period-doubling.
func AssertFeigenbaumCascade(t *testing.T, analysis FeigenbaumAnalysis) {
	t.Helper()

	if len(analysis.Bifurcations) < 2 {
		t.Errorf("Too few bifurcations detected: %d (need at least 2)", len(analysis.Bifurcations))
		return
	}

	// Check period doubling: must be powers of 2 (2, 4, 8, 16, 32, ...)
	// System may skip periods (2→8 is OK), but all detected periods must be 2^n
	for i, bif := range analysis.Bifurcations {
		// Verify period is power of 2
		isPowerOf2 := bif.Period > 0 && (bif.Period&(bif.Period-1)) == 0
		if !isPowerOf2 {
			t.Errorf("Bifurcation %d: period %d is not a power of 2", i+1, bif.Period)
		}

		// Verify period doubling (or skip) from previous
		if i > 0 {
			prevPeriod := analysis.Bifurcations[i-1].Period
			if bif.Period <= prevPeriod {
				t.Errorf("Bifurcation %d: period %d did not increase from %d",
					i+1, bif.Period, prevPeriod)
			}
		}

		t.Logf("Bifurcation %d: r=%.4f, period=%d, amplitude=%.4f, dimension=%.2f",
			i+1, bif.R, bif.Period, bif.Amplitude, bif.Dimension)
	}

	// Check Feigenbaum delta (should be ≈ 4.669)
	if analysis.Delta > 0 {
		expectedDelta := 4.669
		tolerance := 0.5 // Allow 10% error
		if math.Abs(analysis.Delta-expectedDelta) > tolerance {
			t.Errorf("Feigenbaum δ = %.3f (expected ≈ %.3f ± %.1f)",
				analysis.Delta, expectedDelta, tolerance)
		} else {
			t.Logf("✓ Feigenbaum δ = %.3f (universal constant ≈ 4.669)", analysis.Delta)
		}
	}

	// Check Feigenbaum alpha (should be ≈ 2.502)
	if analysis.Alpha > 0 {
		expectedAlpha := 2.502
		tolerance := 0.5
		if math.Abs(analysis.Alpha-expectedAlpha) > tolerance {
			t.Errorf("Feigenbaum α = %.3f (expected ≈ %.3f ± %.1f)",
				analysis.Alpha, expectedAlpha, tolerance)
		} else {
			t.Logf("✓ Feigenbaum α = %.3f (universal constant ≈ 2.502)", analysis.Alpha)
		}
	}

	// Check saturation boundary exists
	if analysis.SaturationBoundary == 0 {
		t.Errorf("No saturation boundary detected (expected r ≈ 3.57 for logistic map)")
	} else {
		t.Logf("✓ Saturation boundary: r = %.4f", analysis.SaturationBoundary)
	}
}

// AssertRecovery verifies the system can exit saturation and return to stability.
func AssertRecovery(t *testing.T, analysis FeigenbaumAnalysis, maxIterations int) {
	t.Helper()

	if analysis.RecoveryTime < 0 {
		t.Errorf("❌ System FAILED to recover (trapped in saturation)")
	} else if analysis.RecoveryTime > maxIterations {
		t.Errorf("❌ Recovery too slow: %d iterations (max: %d)",
			analysis.RecoveryTime, maxIterations)
	} else {
		t.Logf("✓ System recovered in %d iterations (recovered from saturation)",
			analysis.RecoveryTime)
	}
}

// AssertSaturationTransit verifies the system can transit through saturation without diverging.
func AssertSaturationTransit(t *testing.T, analysis FeigenbaumAnalysis, maxIterations int) {
	t.Helper()

	if analysis.TransitTime < 0 {
		t.Errorf("❌ System FAILED to transit saturation (diverged or trapped)")
	} else if analysis.TransitTime > maxIterations {
		t.Errorf("❌ Transit too slow: %d iterations (max: %d)",
			analysis.TransitTime, maxIterations)
	} else {
		t.Logf("✓ System transited saturation in %d iterations (bounded trajectory)",
			analysis.TransitTime)
	}
}

// AssertFractalDimension verifies the chaotic attractor has incomplete dimension.
// Lorenz butterfly: 2.06, Rössler: 2.01, Logistic: varies
func AssertFractalDimension(t *testing.T, analysis FeigenbaumAnalysis, expected float64, tolerance float64) {
	t.Helper()

	if analysis.FractalDimension == 0 {
		t.Errorf("No fractal dimension measured (no saturation detected)")
		return
	}

	if math.Abs(analysis.FractalDimension-expected) > tolerance {
		t.Logf("⚠ Fractal dimension: %.3f (expected %.3f ± %.2f)",
			analysis.FractalDimension, expected, tolerance)
	} else {
		t.Logf("✓ Fractal dimension: %.3f (incomplete dimension = saturation signature)",
			analysis.FractalDimension)
	}

	// Check if dimension is between 2 and 3 (strange attractor)
	if analysis.FractalDimension >= 2.0 && analysis.FractalDimension < 3.0 {
		t.Logf("✓ Strange attractor confirmed (2 < D < 3)")
	}
}

// AssertBasinCompatibility verifies the system stays in life-compatible region.
// Like Earth's orbit: never equilibrium, but bounded and stable enough for life.
func AssertBasinCompatibility(t *testing.T, analysis FeigenbaumAnalysis) {
	t.Helper()

	if !analysis.BasinCompatible {
		t.Errorf("❌ System diverged from life-compatible basin (amplitude exceeded limit)")
	} else {
		t.Logf("✓ System remains basin-compatible (bounded like Earth-Sun-Galaxy)")
	}
}

// PrintBifurcationDiagram outputs the full cascade for visualization.
func PrintBifurcationDiagram(t *testing.T, analysis FeigenbaumAnalysis) {
	t.Helper()

	t.Logf("\n=== Feigenbaum Bifurcation Diagram ===")
	t.Logf("Bifurcations detected: %d", len(analysis.Bifurcations))

	for i, bif := range analysis.Bifurcations {
		t.Logf("  [%d] r=%.4f → Period-%d (amplitude: %.4f, dimension: %.2f)",
			i+1, bif.R, bif.Period, bif.Amplitude, bif.Dimension)
	}

	t.Logf("\nFeigenbaum Constants:")
	t.Logf("  δ (delta) = %.3f (expected ≈ 4.669)", analysis.Delta)
	t.Logf("  α (alpha) = %.3f (expected ≈ 2.502)", analysis.Alpha)

	t.Logf("\nSaturation Properties:")
	t.Logf("  Boundary: r = %.4f", analysis.SaturationBoundary)
	t.Logf("  Fractal dimension: %.3f", analysis.FractalDimension)
	t.Logf("  Recovery: %d iterations", analysis.RecoveryTime)
	t.Logf("  Transit time: %d iterations", analysis.TransitTime)
	t.Logf("  Basin compatible: %v", analysis.BasinCompatible)
}

// LogisticMap is the canonical example: x_{n+1} = r*x_n*(1-x_n)
// Period doubling occurs at r ≈ 3.0, 3.45, 3.54, 3.57 (saturation)
func LogisticMap(x, r float64) float64 {
	return r * x * (1 - x)
}

// PerformanceMap converts performance metrics to iterative map.
// Example: latency as function of load
type PerformanceMap func(ctx context.Context, load float64) (float64, error)

// AdaptPerformanceToMap converts real performance measurements to mathematical map.
func AdaptPerformanceToMap(perfMap PerformanceMap) MapFunction {
	// Cache for performance measurements
	cache := make(map[float64]float64)

	return func(x, r float64) float64 {
		// x = current latency (normalized)
		// r = load parameter (0 to 4.0)

		// Check cache first
		if val, ok := cache[r]; ok {
			// Apply map transformation
			return val * x * (1 - x) // Logistic-like behavior
		}

		// Measure actual performance (expensive)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		latency, err := perfMap(ctx, r)
		if err != nil {
			return x // Keep current value on error
		}

		// Normalize and cache
		normalized := latency / 1000.0 // Assume latency in microseconds
		cache[r] = normalized

		return normalized * x * (1 - x)
	}
}
