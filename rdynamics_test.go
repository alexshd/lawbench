package lawbench

import (
	"testing"
)

// TestRDynamics_Creation verifies initial state.
func TestRDynamics_Creation(t *testing.T) {
	tests := []struct {
		name        string
		initialR    float64
		expectInstability bool
	}{
		{"Stable low", 1.5, false},
		{"Stable high", 2.9, false},
		{"At boundary", 3.0, true},
		{"In instability", 3.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := NewRDynamics(tt.initialR)

			if rd.InitialR != tt.initialR {
				t.Errorf("InitialR = %.4f, want %.4f", rd.InitialR, tt.initialR)
			}

			if rd.CurrentR != tt.initialR {
				t.Errorf("CurrentR = %.4f, want %.4f", rd.CurrentR, tt.initialR)
			}

			if rd.InSaturationZone != tt.expectInstability {
				t.Errorf("InSaturationZone = %v, want %v", rd.InSaturationZone, tt.expectInstability)
			}

			if tt.expectInstability {
				t.Logf("✓ r=%.4f correctly identified as instability", tt.initialR)
			} else {
				t.Logf("✓ r=%.4f correctly identified as stable", tt.initialR)
			}
		})
	}
}

// TestRDynamics_Recovery_PerfectIsolation verifies iterative correction.
func TestRDynamics_Recovery_PerfectIsolation(t *testing.T) {
	// Start in instability: r = 3.5
	rd := NewRDynamics(3.5)

	// Perfect isolation: 100 immutable ops, 0 mutable violations
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   0,
	}

	// Apply iterative recovery until stable
	finalR, iterations := rd.ApplyRecoveryUntilStable(metrics, 20)

	// Should reach stable range (r < 3.0)
	if finalR >= StableDNAConstraint.MaxR {
		t.Errorf("Recovery failed: r=%.4f still ≥ 3.0 after %d iterations", finalR, iterations)
	}

	// Should take multiple iterations (small incremental corrections)
	if iterations < 2 {
		t.Errorf("Expected multiple iterations, got %d (corrections too large = panic risk)", iterations)
	}

	t.Logf("✓ Iterative recovery: r=%.4f → r=%.4f in %d iterations",
		rd.InitialR, finalR, iterations)
	t.Logf("  Perfect isolation achieved stable state with small incremental corrections")
	t.Logf("  Iterations to attractor: %d (each correction ≤ 1/δ ≈ 0.214)", iterations)
}

// TestRDynamics_Recovery_PartialIsolation verifies partial correction.
func TestRDynamics_Recovery_PartialIsolation(t *testing.T) {
	// Start in instability: r = 3.5
	rd := NewRDynamics(3.5)

	// Partial isolation: 100 immutable ops, 20 mutable violations (20% violation rate)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   20,
	}

	newR := rd.ApplyRecovery(metrics)

	// Should correct partially (not fully due to violations)
	correction := rd.InitialR - newR
	if correction <= 0 {
		t.Error("No correction applied")
	}

	// Still in unstable region (partial correction)
	if newR < StableDNAConstraint.MaxR {
		t.Logf("⚠ Partial correction achieved stable range: r=%.4f", newR)
	} else {
		t.Logf("✓ Partial correction: r=%.4f → r=%.4f (corrected %.4f, still in instability)",
			rd.InitialR, newR, correction)
		t.Logf("  20%% isolation violations limited correction effectiveness")
	}
}

// TestRDynamics_Recovery_NoIsolation verifies minimal correction.
func TestRDynamics_Recovery_NoIsolation(t *testing.T) {
	// Start in instability: r = 3.5
	rd := NewRDynamics(3.5)

	// No isolation: 10 immutable ops, 100 mutable violations (1000% violation rate!)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 10,
		MutableSharedState:   100,
	}

	newR := rd.ApplyRecovery(metrics)

	// Should apply minimal correction (isolation too poor)
	correction := rd.InitialR - newR

	if correction > 0.1 {
		t.Errorf("Correction too strong: %.4f (expected minimal with 1000%% violations)", correction)
	}

	t.Logf("✓ Minimal correction: r=%.4f → r=%.4f (corrected %.4f)",
		rd.InitialR, newR, correction)
	t.Logf("  Severe isolation violations prevented effective recovery")
	t.Logf("  Action required: Enforce Law I (Abstract Algebra verification)")
}

// TestRDynamics_FeigenbaumGovernance_CompliantScaling verifies stable scaling.
func TestRDynamics_FeigenbaumGovernance_CompliantScaling(t *testing.T) {
	// Start stable: r = 2.0
	rd := NewRDynamics(2.0)

	// Compliant scaling: 0.20 < 0.214 (within 1/δ limit)
	scalingRatio := 0.20

	newR := rd.ApplyFeigenbaumGovernance(scalingRatio)

	// r should increase slightly but stay in stable range
	increase := newR - rd.InitialR

	if newR >= StableDNAConstraint.MaxR {
		t.Errorf("Compliant scaling pushed r into instability: %.4f ≥ 3.0", newR)
	}

	if increase <= 0 {
		t.Error("Expected r to increase with scaling")
	}

	t.Logf("✓ Compliant scaling: r=%.4f → r=%.4f (+%.6f)",
		rd.InitialR, newR, increase)
	t.Logf("  Scaling ratio %.4f < %.4f (1/δ) - stable", scalingRatio, CriticalityScalingRatio)
}

// TestRDynamics_FeigenbaumGovernance_ViolatingScaling verifies escalation.
func TestRDynamics_FeigenbaumGovernance_ViolatingScaling(t *testing.T) {
	// Start stable but near boundary: r = 2.8
	rd := NewRDynamics(2.8)

	// Violating scaling: 5.0 >> 0.214 (23x over 1/δ limit!)
	scalingRatio := 5.0

	newR := rd.ApplyFeigenbaumGovernance(scalingRatio)

	// r should increase significantly
	increase := newR - rd.InitialR

	if increase <= 0 {
		t.Error("Expected r to increase with scaling")
	}

	if newR >= StableDNAConstraint.MaxR {
		t.Logf("❌ Excessive scaling pushed r into instability: r=%.4f → r=%.4f (+%.6f)",
			rd.InitialR, newR, increase)
		t.Logf("  Scaling ratio %.4f >> %.4f (1/δ) - VIOLATED", scalingRatio, CriticalityScalingRatio)
	} else {
		t.Logf("⚠ Heavy scaling: r=%.4f → r=%.4f (+%.6f, approaching instability)",
			rd.InitialR, newR, increase)
	}
}

// TestCorrectRAfterRecovery_CombinedStrategy verifies both phases.
func TestCorrectRAfterRecovery_CombinedStrategy(t *testing.T) {
	// Start in instability: r = 3.8
	rd := NewRDynamics(3.8)

	// Good isolation: 100 immutable, 5 mutable (5% violations)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   5,
		ScalingRatio:         0.15, // Below 1/δ limit
	}

	// Phase 1: Recovery + Phase 2: Feigenbaum governance
	newR := CorrectRAfterRecovery(&rd, metrics, metrics.ScalingRatio)

	// Should:
	// 1. Correct down from 3.8 (recovery)
	// 2. Then apply small increase (Feigenbaum governance)

	if newR >= rd.InitialR {
		t.Errorf("Combined strategy should reduce r: %.4f → %.4f",
			rd.InitialR, newR)
	}

	if rd.RecoveryEvents != 1 {
		t.Errorf("Expected 1 recovery event, got %d", rd.RecoveryEvents)
	}

	t.Logf("✓ Combined strategy: r=%.4f → r=%.4f",
		rd.InitialR, newR)
	t.Logf("  Phase 1 (Recovery): Active correction via Law I")
	t.Logf("  Phase 2 (Feigenbaum): Preventive governance via Law III")

	if newR < StableDNAConstraint.MaxR {
		t.Logf("  ✓ Success: System returned to stable equilibrium")
	} else {
		t.Logf("  ⚠ Partial success: Still in instability, requires more correction")
	}
}

// TestPerpetualStructuralIntegrity_Stable verifies Σ_R for stable system.
func TestPerpetualStructuralIntegrity_Stable(t *testing.T) {
	rd := NewRDynamics(2.0)

	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   0,
		ScalingRatio:         0.20,
	}

	err := PerpetualStructuralIntegrity(&rd, metrics)
	if err != nil {
		t.Errorf("Σ_R violation for stable system: %v", err)
	}

	t.Logf("✓ Σ_R satisfied: r=%.4f, scaling=%.4f", rd.CurrentR, metrics.ScalingRatio)
}

// TestPerpetualStructuralIntegrity_InstabilityZone verifies Σ_R rejects instability.
func TestPerpetualStructuralIntegrity_InstabilityZone(t *testing.T) {
	rd := NewRDynamics(3.5)

	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 10,
		MutableSharedState:   50,
		ScalingRatio:         0.20,
	}

	err := PerpetualStructuralIntegrity(&rd, metrics)
	if err == nil {
		t.Error("Σ_R should reject system in unstable region")
	}

	t.Logf("✓ Σ_R correctly rejected: %v", err)
}

// TestPerpetualStructuralIntegrity_ScalingViolation verifies Σ_R rejects excessive scaling.
func TestPerpetualStructuralIntegrity_ScalingViolation(t *testing.T) {
	rd := NewRDynamics(2.0)

	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   0,
		ScalingRatio:         5.0, // 23x over limit!
	}

	err := PerpetualStructuralIntegrity(&rd, metrics)
	if err == nil {
		t.Error("Σ_R should reject excessive scaling ratio")
	}

	t.Logf("✓ Σ_R correctly rejected: %v", err)
}

// TestSimulateRTrajectory_StableEvolution verifies prediction model.
func TestSimulateRTrajectory_StableEvolution(t *testing.T) {
	events := []REvent{
		{
			Type:         "scaling",
			ScalingRatio: 0.10,
			Description:  "Add 10 LOC to extensible, 100 LOC to core",
		},
		{
			Type:         "scaling",
			ScalingRatio: 0.15,
			Description:  "Add 15 LOC to extensible, 100 LOC to core",
		},
		{
			Type:         "scaling",
			ScalingRatio: 0.20,
			Description:  "Add 20 LOC to extensible, 100 LOC to core",
		},
	}

	trajectory := SimulateRTrajectory(2.0, events)

	if len(trajectory.R) != len(events)+1 {
		t.Errorf("Expected %d r values, got %d", len(events)+1, len(trajectory.R))
	}

	t.Log("\n=== R Trajectory Simulation ===")
	t.Logf("Initial: r = %.6f", trajectory.R[0])

	for i, event := range events {
		t.Logf("Event %d: %s", i+1, event.Description)
		t.Logf("  r = %.6f → %.6f (Δr = %+.6f)",
			trajectory.R[i], trajectory.R[i+1], trajectory.R[i+1]-trajectory.R[i])
	}

	finalR := trajectory.R[len(trajectory.R)-1]
	if finalR >= StableDNAConstraint.MaxR {
		t.Errorf("❌ Trajectory led to instability: r=%.4f ≥ 3.0", finalR)
	} else {
		t.Logf("\n✓ Trajectory stable: final r=%.6f < 3.0", finalR)
	}
}

// TestSimulateRTrajectory_InstabilityThenRecovery verifies correction cycle.
func TestSimulateRTrajectory_InstabilityThenRecovery(t *testing.T) {
	events := []REvent{
		{
			Type: "violation",
			Metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified: 10,
				MutableSharedState:   50,
			},
			Description: "Introduce shared mutable state (Law I violation)",
		},
		{
			Type:         "scaling",
			ScalingRatio: 1.0,
			Description:  "Add complexity without strengthening core",
		},
		{
			Type: "recovery",
			Metrics: SystemIntegrityMetrics{
				ImmutableOpsVerified: 100,
				MutableSharedState:   0,
			},
			Description: "Enforce Law I: Convert to immutable operations",
		},
		{
			Type:         "scaling",
			ScalingRatio: 0.15,
			Description:  "Compliant scaling after recovery",
		},
	}

	trajectory := SimulateRTrajectory(2.0, events)

	t.Log("\n=== Instability → Recovery → Stable Trajectory ===")
	t.Logf("Initial: r = %.6f (stable)", trajectory.R[0])

	for i, event := range events {
		delta := trajectory.R[i+1] - trajectory.R[i]
		r := trajectory.R[i+1]

		status := "stable"
		if r >= StableDNAConstraint.MaxR {
			status = "CHAOS"
		}

		t.Logf("\nEvent %d: %s", i+1, event.Description)
		t.Logf("  r = %.6f → %.6f (Δr = %+.6f) [%s]",
			trajectory.R[i], r, delta, status)
	}

	// Verify recovery worked
	beforeDefib := trajectory.R[2]
	afterDefib := trajectory.R[3]

	if afterDefib >= beforeDefib {
		t.Errorf("Recovery failed to reduce r: %.4f → %.4f",
			beforeDefib, afterDefib)
	}

	t.Logf("\n✓ Recovery effective: r=%.6f → r=%.6f (corrected %.6f)",
		beforeDefib, afterDefib, beforeDefib-afterDefib)
}

// TestRDynamics_Philosophy documents the complete r management model.
func TestRDynamics_Philosophy(t *testing.T) {
	t.Log("\n=== The Complete R Management Model ===")
	t.Log("")
	t.Log("Coupling Parameter (r): Measures system interdependence")
	t.Logf("  DNA Constraint: %.1f < r < %.1f (Stable Equilibrium)",
		StableDNAConstraint.MinR, StableDNAConstraint.MaxR)
	t.Logf("  r ≥ %.1f: Period-doubling cascade → Instability → Geometric failure",
		StableDNAConstraint.MaxR)
	t.Log("")
	t.Log("Phase I: Correcting r (Recovery)")
	t.Log("  When: r ≥ 3.0 (unstable region)")
	t.Log("  How: Enforce Law I (Isolation via Abstract Algebra)")
	t.Log("  Result: r_instability → r_stable")
	t.Log("  Formula: r_corrected = r_current - correction_factor")
	t.Log("    where correction_factor = instability_depth / (1 + isolation_violations)")
	t.Log("")
	t.Log("Phase II: Governing r (Feigenbaum Constraint)")
	t.Log("  When: Always (preventive)")
	t.Log("  How: Enforce Law III (Scaling ≤ 1/δ)")
	t.Logf("  Result: Δr bounded by %.4f (1/δ)", CriticalityScalingRatio)
	t.Log("  Formula: r_next = r_current + (scaling_ratio / δ²)")
	t.Log("")
	t.Log("Perpetual Structural Integrity (Σ_R):")
	t.Log("  Σ_R ≡ Enforce { 1 < r_eff(x, ΔC) < 3 }")
	t.Log("       via     { ΔComplexity/ΔCore ≤ 1/δ }")
	t.Log("")
	t.Log("Three-Law Synthesis:")
	t.Log("  Law I (Isolation):   Suppresses base r (recovery)")
	t.Log("  Law II (Supervision): Stabilizes r under failure (resilience)")
	t.Log("  Law III (Scaling):    Bounds r growth rate (Feigenbaum)")
	t.Log("")
	t.Logf("Together: r starts low (Law I), stays stable (Law II), grows slowly (Law III/1/δ)")
}
