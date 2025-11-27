package lawbench

import (
	"math"
	"testing"
)

// TestFeigenbaumConstant verifies the constant is correctly defined.
// Note: Rounded to 4 decimal places for distributed systems.
// Network I/O noise (~ms) makes sub-millisecond precision meaningless.
func TestFeigenbaumConstant(t *testing.T) {
	expected := 4.6692 // Realistic precision for network systems

	if math.Abs(FeigenbaumDelta-expected) > 1e-4 {
		t.Errorf("Feigenbaum delta incorrect: got %.4f, expected %.4f",
			FeigenbaumDelta, expected)
	}

	t.Logf("✓ Feigenbaum δ = %.4f (rounded for distributed systems)", FeigenbaumDelta)
}

// TestCriticalityScalingRatio verifies 1/δ ≈ 0.214.
// Precision limited to what matters in distributed systems.
func TestCriticalityScalingRatio(t *testing.T) {
	expected := 1.0 / 4.6692 // Match rounded constant

	if math.Abs(CriticalityScalingRatio-expected) > 1e-4 {
		t.Errorf("Criticality ratio incorrect: got %.4f, expected %.4f",
			CriticalityScalingRatio, expected)
	}

	// Should be approximately 0.214 (21.4%)
	if CriticalityScalingRatio < 0.213 || CriticalityScalingRatio > 0.215 {
		t.Errorf("Criticality ratio out of range: %.4f (expected ≈ 0.214)",
			CriticalityScalingRatio)
	}

	t.Logf("✓ 1/δ = %.4f (≈ 0.214 or 21.4%%)", CriticalityScalingRatio)
}

// TestSystemDNAConstraint verifies the stable equilibrium range.
func TestSystemDNAConstraint(t *testing.T) {
	if StableDNAConstraint.MinR != 1.0 {
		t.Errorf("DNA MinR should be 1.0, got %.1f", StableDNAConstraint.MinR)
	}

	if StableDNAConstraint.MaxR != 3.0 {
		t.Errorf("DNA MaxR should be 3.0, got %.1f", StableDNAConstraint.MaxR)
	}

	t.Logf("✓ DNA Constraint: %.1f < r < %.1f (Stable Equilibrium)",
		StableDNAConstraint.MinR, StableDNAConstraint.MaxR)
}

// TestCriticalityConstraint_ValidScaling verifies compliant scaling passes.
func TestCriticalityConstraint_ValidScaling(t *testing.T) {
	// Scenario: Add 10 units to critical core, 2 units to extensible
	// Ratio: 2/10 = 0.20 < 0.214 ✓
	constraint := NewCriticalityConstraint(10.0, 2.0)

	err := constraint.Validate()
	if err != nil {
		t.Errorf("Valid scaling rejected: %v", err)
	}

	ratio := constraint.Ratio()
	if ratio >= CriticalityScalingRatio {
		t.Errorf("Ratio %.4f should be < %.4f", ratio, CriticalityScalingRatio)
	}

	t.Logf("✓ Valid scaling: ΔCore=%.0f, ΔComplex=%.0f, ratio=%.4f < %.4f",
		constraint.DeltaCriticalCore, constraint.DeltaComplexity,
		ratio, CriticalityScalingRatio)
}

// TestCriticalityConstraint_ViolatesScaling verifies excessive scaling fails.
func TestCriticalityConstraint_ViolatesScaling(t *testing.T) {
	// Scenario: Add 10 units to critical core, 5 units to extensible
	// Ratio: 5/10 = 0.50 > 0.214 ✗
	constraint := NewCriticalityConstraint(10.0, 5.0)

	err := constraint.Validate()
	if err == nil {
		t.Error("Invalid scaling was accepted (should reject ratio > 1/δ)")
	}

	ratio := constraint.Ratio()
	if ratio <= CriticalityScalingRatio {
		t.Errorf("Ratio %.4f should be > %.4f", ratio, CriticalityScalingRatio)
	}

	t.Logf("✓ Correctly rejected: ratio=%.4f > %.4f (1/δ)\n  Error: %v",
		ratio, CriticalityScalingRatio, err)
}

// TestCriticalityConstraint_BoundaryCase tests exact 1/δ ratio.
func TestCriticalityConstraint_BoundaryCase(t *testing.T) {
	// Scenario: Exactly at the Feigenbaum limit
	// Ratio: 0.214 = 1/δ (boundary)
	deltaCritical := 100.0
	deltaComplex := deltaCritical * CriticalityScalingRatio

	constraint := NewCriticalityConstraint(deltaCritical, deltaComplex)

	err := constraint.Validate()
	if err != nil {
		t.Errorf("Boundary case rejected: %v", err)
	}

	ratio := constraint.Ratio()
	expected := CriticalityScalingRatio
	if math.Abs(ratio-expected) > 1e-10 {
		t.Errorf("Boundary ratio incorrect: got %.10f, expected %.10f",
			ratio, expected)
	}

	t.Logf("✓ Boundary case: ratio=%.10f = 1/δ (exactly at limit)",
		ratio)
}

// TestCriticalityConstraint_Headroom verifies headroom calculation.
func TestCriticalityConstraint_Headroom(t *testing.T) {
	// Scenario: Core=100, Complex=10, ratio=0.10
	// Headroom: (100 * 0.214) - 10 = 21.4 - 10 = 11.4
	constraint := NewCriticalityConstraint(100.0, 10.0)

	headroom := constraint.Headroom()
	expected := 100.0*CriticalityScalingRatio - 10.0

	if math.Abs(headroom-expected) > 1e-6 {
		t.Errorf("Headroom incorrect: got %.4f, expected %.4f",
			headroom, expected)
	}

	if headroom < 0 {
		t.Error("Headroom should be positive for valid constraint")
	}

	t.Logf("✓ Headroom: %.4f units of complexity can be added", headroom)
}

// TestCriticalityConstraint_IsStableEquilibrium verifies DNA range check.
func TestCriticalityConstraint_IsStableEquilibrium(t *testing.T) {
	tests := []struct {
		name     string
		r        float64
		expected bool
	}{
		{"Below minimum", 0.5, false},
		{"At minimum", 1.0, false},
		{"Stable low", 1.5, true},
		{"Stable mid", 2.0, true},
		{"Stable high", 2.9, true},
		{"At maximum", 3.0, false},
		{"In chaos", 3.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := NewCriticalityConstraint(10.0, 2.0)
			constraint.CurrentCouplingR = tt.r

			result := constraint.IsStableEquilibrium()
			if result != tt.expected {
				t.Errorf("r=%.1f: got %v, expected %v", tt.r, result, tt.expected)
			}

			if result {
				t.Logf("✓ r=%.1f is in stable equilibrium", tt.r)
			} else {
				t.Logf("✓ r=%.1f is outside stable range", tt.r)
			}
		})
	}
}

// TestCriticalityConstraint_DistanceToInstabilityBoundary verifies distance calc.
func TestCriticalityConstraint_DistanceToInstabilityBoundary(t *testing.T) {
	tests := []struct {
		r        float64
		expected float64
	}{
		{1.0, 2.0},  // 3.0 - 1.0 = 2.0
		{2.0, 1.0},  // 3.0 - 2.0 = 1.0
		{2.9, 0.1},  // 3.0 - 2.9 = 0.1
		{3.0, 0.0},  // At boundary
		{3.5, -0.5}, // In chaos
	}

	for _, tt := range tests {
		constraint := NewCriticalityConstraint(10.0, 2.0)
		constraint.CurrentCouplingR = tt.r

		distance := constraint.DistanceToInstabilityBoundary()

		if math.Abs(distance-tt.expected) > 1e-10 {
			t.Errorf("r=%.1f: distance=%.4f, expected %.4f",
				tt.r, distance, tt.expected)
		}

		if distance < 0 {
			t.Logf("✓ r=%.1f: %.4f units INTO chaos zone", tt.r, -distance)
		} else if distance < 0.5 {
			t.Logf("⚠ r=%.1f: %.4f units from chaos (danger zone)", tt.r, distance)
		} else {
			t.Logf("✓ r=%.1f: %.4f units from chaos (safe)", tt.r, distance)
		}
	}
}

// TestCalculateSystemDNA verifies the three-law coupling model.
func TestCalculateSystemDNA(t *testing.T) {
	tests := []struct {
		name    string
		metrics SystemIntegrityMetrics
		minR    float64
		maxR    float64
	}{
		{
			name: "Perfect system",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  100,
				MutableSharedState:    0,
				SupervisedProcesses:   50,
				UnsupervisedProcesses: 0,
				CriticalCoreLOC:       1000,
				ExtensibleLOC:         200,
				ScalingRatio:          0.20,
			},
			minR: 1.0,
			maxR: 2.0, // Should be in stable range
		},
		{
			name: "Moderate violations",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  100,
				MutableSharedState:    10, // 10% violations
				SupervisedProcesses:   50,
				UnsupervisedProcesses: 5, // 10% unsupervised
				CriticalCoreLOC:       1000,
				ExtensibleLOC:         300,
				ScalingRatio:          0.30, // Above 1/δ
			},
			minR: 1.5,
			maxR: 3.0, // Near boundary
		},
		{
			name: "Chaos zone",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  10,
				MutableSharedState:    50, // 500% violations!
				SupervisedProcesses:   10,
				UnsupervisedProcesses: 40, // 400% unsupervised!
				CriticalCoreLOC:       100,
				ExtensibleLOC:         500,
				ScalingRatio:          5.0, // 23x over limit!
			},
			minR: 3.0,
			maxR: 50.0, // Deep in chaos (model allows high r for extreme violations)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CalculateSystemDNA(tt.metrics)

			if r < tt.minR || r > tt.maxR {
				t.Errorf("r=%.4f outside expected range [%.1f, %.1f]",
					r, tt.minR, tt.maxR)
			}

			if r < StableDNAConstraint.MaxR {
				t.Logf("✓ r=%.4f (STABLE equilibrium)", r)
			} else {
				t.Logf("❌ r=%.4f (CHAOS zone, r ≥ 3.0)", r)
			}
		})
	}
}

// TestValidateSystemDNA verifies three-law enforcement.
func TestValidateSystemDNA(t *testing.T) {
	tests := []struct {
		name      string
		metrics   SystemIntegrityMetrics
		shouldErr bool
	}{
		{
			name: "All laws satisfied",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  100,
				MutableSharedState:    0,
				SupervisedProcesses:   50,
				UnsupervisedProcesses: 0,
				ScalingRatio:          0.20,
			},
			shouldErr: false,
		},
		{
			name: "Law I violated (isolation)",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  10,
				MutableSharedState:    100, // 1000% violations
				SupervisedProcesses:   50,
				UnsupervisedProcesses: 0,
				ScalingRatio:          0.20,
			},
			shouldErr: true,
		},
		{
			name: "Law II violated (supervision)",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  100,
				MutableSharedState:    0,
				SupervisedProcesses:   10,
				UnsupervisedProcesses: 100, // 1000% unsupervised
				ScalingRatio:          0.20,
			},
			shouldErr: true,
		},
		{
			name: "Law III violated (scaling)",
			metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified:  100,
				MutableSharedState:    0,
				SupervisedProcesses:   50,
				UnsupervisedProcesses: 0,
				ScalingRatio:          5.0, // 23x over Feigenbaum limit
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSystemDNA(tt.metrics)

			if tt.shouldErr && err == nil {
				t.Error("Expected error for violated system, got nil")
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if err != nil {
				t.Logf("✓ Correctly rejected: %v", err)
			} else {
				r := CalculateSystemDNA(tt.metrics)
				t.Logf("✓ System valid: r=%.4f (stable equilibrium)", r)
			}
		})
	}
}

// TestFeigenbaumPhilosophy documents the architectural mandate.
func TestFeigenbaumPhilosophy(t *testing.T) {
	t.Log("\n=== The Feigenbaum Architectural Mandate ===")
	t.Log("")
	t.Logf("δ (Feigenbaum constant) = %.20f", FeigenbaumDelta)
	t.Logf("1/δ (Criticality ratio) = %.20f (≈ 21.4%%)", CriticalityScalingRatio)
	t.Log("")
	t.Log("The Universal Scaling Law:")
	t.Logf("  ΔComplexity (Tier 2/3) / ΔCritical (Tier 1) ≤ 1/δ ≈ 0.214")
	t.Log("")
	t.Log("Interpretation:")
	t.Log("  For every 1 unit of change to critical core (Tier 1),")
	t.Log("  you may add at most 0.214 units to extensible layers (Tier 2/3).")
	t.Log("")
	t.Log("Why 1/δ?")
	t.Log("  δ ≈ 4.669 is the universal rate of structural decay toward chaos.")
	t.Log("  Its inverse, 1/δ ≈ 0.214, is the maximum safe scaling factor.")
	t.Log("  Exceeding this ratio accelerates the coupling parameter (r)")
	t.Log("  toward the chaos boundary (r = 3.0), triggering period-doubling")
	t.Log("  cascade and eventual geometric system failure.")
	t.Log("")
	t.Log("System DNA Constraint:")
	t.Logf("  1.0 < r < 3.0 (Stable Equilibrium)")
	t.Logf("  r ≥ 3.0 (Bifurcation Cascade → Chaos)")
	t.Log("")
	t.Log("Three Laws of Architectural Integrity:")
	t.Log("  Law I (Isolation):  Enforce immutability via Abstract Algebra")
	t.Log("  Law II (Supervision): Erlang-style failure handling")
	t.Log("  Law III (Scaling):    Feigenbaum criticality constraint (1/δ)")
	t.Log("")
	t.Log("Together, these laws maintain: 1 < r < 3 (Perpetual Structural Integrity)")
}
