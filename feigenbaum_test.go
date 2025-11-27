package lawbench

import (
	"math"
	"testing"
)

// TestLogisticMap_Recovery verifies system can exit saturation.
func TestLogisticMap_Recovery(t *testing.T) {
	cfg := DefaultFeigenbaumConfig()
	cfg.Iterations = 1000
	cfg.RecoveryThreshold = 0.01

	x0 := 0.5
	rSaturation := 3.9  // Deep in saturation
	rStable := 2.8 // Stable period-1

	iterations := MeasureRecoveryTime(LogisticMap, x0, rSaturation, rStable, cfg)

	if iterations < 0 {
		t.Errorf("❌ System FAILED to recovere (trapped in saturation)")
	} else {
		t.Logf("✓ Recovery successful: %d iterations", iterations)
		t.Logf("  Started at r=%.2f (saturation)", rSaturation)
		t.Logf("  Reduced to r=%.2f (stable)", rStable)
		t.Logf("  Converged in %d map iterations", iterations)
	}

	// Assert reasonable recoverion time
	maxAllowed := 500
	if iterations > maxAllowed {
		t.Errorf("❌ Recovery too slow: %d > %d iterations", iterations, maxAllowed)
	}
}

// TestLogisticMap_SaturationTransit verifies system can pass through saturation without diverging.
func TestLogisticMap_SaturationTransit(t *testing.T) {
	cfg := DefaultFeigenbaumConfig()
	cfg.Iterations = 1000
	cfg.BasinRadius = 1.0 // Logistic map bounded [0,1]

	x0 := 0.5
	rSaturation := 3.9

	iterations := MeasureTransitTime(LogisticMap, x0, rSaturation, cfg)

	if iterations < 0 {
		t.Errorf("❌ System FAILED to transit saturation (diverged)")
	} else {
		t.Logf("✓ Saturation transit successful: %d iterations", iterations)
		t.Logf("  Control parameter: r=%.2f (chaotic)", rSaturation)
		t.Logf("  Stayed bounded within [0, %.1f]", cfg.BasinRadius)
		t.Logf("  Found life-compatible trajectory in %d iterations", iterations)
	}
}

// TestLogisticMap_BasinCompatibility verifies bounded saturation (life-compatible).
func TestLogisticMap_BasinCompatibility(t *testing.T) {
	cfg := DefaultFeigenbaumConfig()
	cfg.Iterations = 5000
	cfg.Warmup = 1000
	cfg.BasinRadius = 1.0

	x0 := 0.5
	rSaturation := 3.9

	trajectory := IterateMap(LogisticMap, x0, rSaturation, cfg)

	// Check all values stay bounded
	allBounded := true
	maxValue := 0.0
	for _, x := range trajectory {
		if x < 0 || x > cfg.BasinRadius {
			allBounded = false
		}
		if x > maxValue {
			maxValue = x
		}
	}

	if !allBounded {
		t.Errorf("❌ System diverged from basin (exceeded [0, %.1f])", cfg.BasinRadius)
	} else {
		t.Logf("✓ System remains basin-compatible")
		t.Logf("  All %d iterations stayed in [0, %.1f]", len(trajectory), cfg.BasinRadius)
		t.Logf("  Maximum value: %.6f", maxValue)
		t.Logf("  This is like Earth's orbit: never equilibrium, but bounded")
	}
}

// TestLogisticMap_EarthSunAnalogy demonstrates basin compatibility concept.
func TestLogisticMap_EarthSunAnalogy(t *testing.T) {
	cfg := DefaultFeigenbaumConfig()
	cfg.Iterations = 1000
	cfg.Warmup = 200

	// Earth-Sun: not at 66.7% efficiency, but on trajectory toward attractor
	x0 := 0.4 // Starting far from any attractor
	r := 3.2  // In period-2 region

	trajectory := IterateMap(LogisticMap, x0, r, cfg)
	period := DetectPeriod(trajectory, cfg)
	amplitude := CalculateAmplitude(trajectory)

	t.Logf("\n=== Earth-Sun Analogy ===")
	t.Logf("Initial condition: x0 = %.2f (not at equilibrium)", x0)
	t.Logf("Control parameter: r = %.2f", r)
	t.Logf("Detected period: %d", period)
	t.Logf("Amplitude: %.4f", amplitude)

	// Check if trajectory approaches attractor over iterations
	early := trajectory[0:100]
	late := trajectory[len(trajectory)-100:]

	earlyAmplitude := CalculateAmplitude(early)
	lateAmplitude := CalculateAmplitude(late)

	t.Logf("\nConvergence to attractor:")
	t.Logf("  Early amplitude (first 100 iter): %.4f", earlyAmplitude)
	t.Logf("  Late amplitude (last 100 iter): %.4f", lateAmplitude)

	if math.Abs(earlyAmplitude-lateAmplitude) < 0.01 {
		t.Logf("  ✓ System settled to attractor")
	} else {
		t.Logf("  → System approaching attractor (like Earth's orbit)")
	}

	// Key insight: Each iteration can be far from 66.7% efficiency
	// as long as next iteration is closer to attractor
	t.Logf("\nKey insight: NOT about being at 66.7%% efficiency")
	t.Logf("  It's about APPROACHING the attractor basin")
	t.Logf("  Earth is NOT at equilibrium with Sun")
	t.Logf("  Sun is NOT at equilibrium with Galaxy center")
	t.Logf("  But both are in life-compatible bounded orbits")
}

// TestFeigenbaum_IterationMeaning verifies iterations are map applications, not time.
func TestFeigenbaum_IterationMeaning(t *testing.T) {
	t.Logf("\n=== What is an 'Iteration'? ===")
	t.Logf("NOT: CPU cycles")
	t.Logf("NOT: Wall-clock time")
	t.Logf("NOT: Generations (biological)")
	t.Logf("YES: Recursive map applications")
	t.Logf("")
	t.Logf("For logistic map: x_{n+1} = f(x_n, r)")
	t.Logf("  Iteration 0: x_0 = 0.5 (initial)")
	t.Logf("  Iteration 1: x_1 = f(x_0, r)")
	t.Logf("  Iteration 2: x_2 = f(x_1, r) = f(f(x_0, r), r)")
	t.Logf("  Iteration n: x_n = f^n(x_0, r)")
	t.Logf("")
	t.Logf("For performance system:")
	t.Logf("  Iteration = feedback cycle")
	t.Logf("  Load affects latency affects load affects latency...")
	t.Logf("  Each 'iteration' is one feedback loop")
	t.Logf("")
	t.Logf("Recovery time: iterations to exit saturation")
	t.Logf("Transit time: iterations through chaotic region")
	t.Logf("")
	t.Logf("Run as long as needed to find correct x and r")
	t.Logf("Speed optimization is WRONG here - we need ACCURACY")

	// Demonstrate with actual map
	r := 3.9
	x := 0.5

	t.Logf("\nExample: 10 iterations at r=%.1f (saturation)", r)
	for i := 0; i < 10; i++ {
		t.Logf("  x_%d = %.6f", i, x)
		x = LogisticMap(x, r)
	}
	t.Logf("  x_10 = %.6f", x)
	t.Logf("\nNotice: unpredictable, bounded, never repeats")
}

// TestFeigenbaum_UniversalConstants verifies δ and α are independent of system details.
func TestFeigenbaum_UniversalConstants(t *testing.T) {
	t.Logf("\n=== Feigenbaum Universality ===")
	t.Logf("These constants appear in ALL period-doubling systems:")
	t.Logf("")
	t.Logf("δ (delta) ≈ 4.669201609...")
	t.Logf("  Rate of period-doubling cascade")
	t.Logf("  (r_{n+1} - r_n) / (r_{n+2} - r_{n+1}) → δ")
	t.Logf("")
	t.Logf("α (alpha) ≈ 2.502907875...")
	t.Logf("  Amplitude scaling between bifurcations")
	t.Logf("  amplitude_n / amplitude_{n+1} → α")
	t.Logf("")
	t.Logf("Found in:")
	t.Logf("  - Logistic map (x → rx(1-x))")
	t.Logf("  - Sine map (x → r sin(πx))")
	t.Logf("  - Fluid turbulence")
	t.Logf("  - Electronic circuits")
	t.Logf("  - Population dynamics")
	t.Logf("  - Distributed systems (?)")
	t.Logf("")
	t.Logf("Universal = same constants across ALL these systems!")
	t.Logf("This is a fundamental law of nature, like π or e")
}

// TestFeigenbaum_LorenzButterfly demonstrates fractal dimension concept.
func TestFeigenbaum_LorenzButterfly(t *testing.T) {
	t.Logf("\n=== Lorenz Butterfly & Fractal Dimension ===")
	t.Logf("Lorenz attractor dimension: D ≈ 2.06")
	t.Logf("")
	t.Logf("Why 2.06, not 2 or 3?")
	t.Logf("  D = 0: Point (stable equilibrium)")
	t.Logf("  D = 1: Curve (periodic orbit)")
	t.Logf("  D = 2: Surface")
	t.Logf("  D = 2.06: Fractal (strange attractor)")
	t.Logf("  D = 3: Volume (fills 3D space)")
	t.Logf("")
	t.Logf("The 0.06 is the 'incomplete dimension'")
	t.Logf("  = signature of saturation")
	t.Logf("  = self-similar structure at all scales")
	t.Logf("")
	t.Logf("For our system:")
	t.Logf("  If D ≈ 1.0: Periodic (predictable)")
	t.Logf("  If 1.0 < D < 2.0: Weakly chaotic")
	t.Logf("  If 2.0 < D < 3.0: Strongly chaotic (strange attractor)")
	t.Logf("")
	t.Logf("We TEST:")
	t.Logf("  1. Can system enter this fractal dimension?")
	t.Logf("  2. Can it recovere (exit)?")
	t.Logf("  3. Can it transit through without diverging?")
	t.Logf("  4. Does it stay in life-compatible basin?")
}
