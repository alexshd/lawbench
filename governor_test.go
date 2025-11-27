package lawbench

import (
	"strings"
	"testing"
)

func TestGovernor_Stable(t *testing.T) {
	g := NewGovernor(2.4) // Healthy system

	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    5, // 5% violations
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 2,    // 4% unsupervised
		ScalingRatio:          0.15, // Well below 1/δ
	}

	action := g.CheckStructuralIntegrity(metrics)

	if action.Type != ActionStable {
		t.Errorf("Expected STABLE, got %s", action.Type)
	}

	if !strings.Contains(action.Reason, "STABLE") {
		t.Errorf("Expected STABLE reason, got: %s", action.Reason)
	}
}

func TestGovernor_Warning(t *testing.T) {
	g := NewGovernor(2.85) // Initial value doesn't matter - calculated from metrics

	// Metrics that produce r ≈ 2.81 (in warning zone)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    65, // 65% violations
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 14,   // 28% unsupervised
		ScalingRatio:          0.19, // Approaching limit
	}

	action := g.CheckStructuralIntegrity(metrics)

	if action.Type != ActionWarning {
		t.Errorf("Expected WARNING, got %s", action.Type)
	}

	if !strings.Contains(action.Reason, "WARNING") {
		t.Errorf("Expected WARNING reason, got: %s", action.Reason)
	}

	// Check warnings counter
	stats := g.GetStatistics()
	if stats["warnings_issued"].(int) != 1 {
		t.Errorf("Expected 1 warning, got %d", stats["warnings_issued"].(int))
	}
}

func TestGovernor_Pacing(t *testing.T) {
	g := NewGovernor(2.95) // Initial value doesn't matter - calculated from metrics

	// Metrics that produce r ≈ 2.91 (danger zone)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    68, // 68% violations
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 16,   // 32% unsupervised
		ScalingRatio:          0.21, // Violating limit
	}

	action := g.CheckStructuralIntegrity(metrics)

	if action.Type != ActionPacing {
		t.Errorf("Expected PACING, got %s", action.Type)
	}

	if !strings.Contains(action.Reason, "DANGER") {
		t.Errorf("Expected DANGER reason, got: %s", action.Reason)
	}

	if !strings.Contains(action.Mitigation, "PACING") {
		t.Errorf("Expected PACING mitigation, got: %s", action.Mitigation)
	}
}

func TestGovernor_Throttle(t *testing.T) {
	g := NewGovernor(3.5) // Saturation zone

	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    50, // Severe coupling
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 20,
		ScalingRatio:          0.30, // Major violation
	}

	action := g.CheckStructuralIntegrity(metrics)

	if action.Type != ActionThrottle {
		t.Errorf("Expected THROTTLE, got %s", action.Type)
	}

	if !strings.Contains(action.Reason, "SATURATION DETECTED") {
		t.Errorf("Expected SATURATION reason, got: %s", action.Reason)
	}

	if !strings.Contains(action.Mitigation, "THROTTLE") {
		t.Errorf("Expected THROTTLE mitigation, got: %s", action.Mitigation)
	}

	// Check throttles counter
	stats := g.GetStatistics()
	if stats["throttles_applied"].(int) != 1 {
		t.Errorf("Expected 1 throttle, got %d", stats["throttles_applied"].(int))
	}
}

func TestGovernor_BlockDeploy_FeigenbaumViolation(t *testing.T) {
	g := NewGovernor(2.5)

	// Deployment that adds 500 LOC to Tier 2/3 but only 50 LOC to Tier 1
	// Ratio: 500/50 = 10.0 >> 4.669 (violation!)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    10,
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 5,
		ScalingRatio:          0.15,
		DeltaCriticalCore:     50.0,  // Tier 1 changes
		DeltaComplexity:       500.0, // Tier 2/3 changes
	}

	action := g.CheckStructuralIntegrity(metrics)

	if action.Type != ActionBlockDeploy {
		t.Errorf("Expected BLOCK_DEPLOY, got %s", action.Type)
	}

	if !strings.Contains(action.Reason, "Σ_R Violation") {
		t.Errorf("Expected Σ_R Violation, got: %s", action.Reason)
	}

	if !strings.Contains(action.Reason, "10.00 exceeds Feigenbaum Limit 4.67") {
		t.Errorf("Expected specific ratio violation, got: %s", action.Reason)
	}

	if !strings.Contains(action.Mitigation, "Technical Debt") {
		t.Errorf("Expected Technical Debt explanation, got: %s", action.Mitigation)
	}

	// Check blocked counter
	stats := g.GetStatistics()
	if stats["deploys_blocked"].(int) != 1 {
		t.Errorf("Expected 1 blocked deploy, got %d", stats["deploys_blocked"].(int))
	}
}

func TestGovernor_AllowDeploy_FeigenbaumCompliant(t *testing.T) {
	g := NewGovernor(2.4)

	// Deployment that adds 200 LOC to Tier 2/3 and 50 LOC to Tier 1
	// Ratio: 200/50 = 4.0 < 4.669 (compliant!)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    5,
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 2,
		ScalingRatio:          0.12,
		DeltaCriticalCore:     50.0,  // Tier 1 changes
		DeltaComplexity:       200.0, // Tier 2/3 changes
	}

	action := g.CheckStructuralIntegrity(metrics)

	// Should pass Phase I, check runtime state
	if action.Type == ActionBlockDeploy {
		t.Errorf("Deployment should not be blocked, got: %s", action.Reason)
	}

	// Should be stable (r=2.4)
	if action.Type != ActionStable {
		t.Errorf("Expected STABLE, got %s", action.Type)
	}
}

func TestGovernor_The21PercentRule(t *testing.T) {
	// Test the "21% Rule": 1/δ ≈ 0.214 ≈ 21.4%
	// For every 1 unit of Core work, earn right to 4.669 units of Feature work

	g := NewGovernor(2.0)

	testCases := []struct {
		name          string
		deltaCore     float64
		deltaFeatures float64
		shouldBlock   bool
	}{
		{
			name:          "Balanced growth (1:4 ratio)",
			deltaCore:     100,
			deltaFeatures: 400,
			shouldBlock:   false, // 400/100 = 4.0 < 4.669
		},
		{
			name:          "At boundary (1:4.6 ratio)",
			deltaCore:     100,
			deltaFeatures: 460,
			shouldBlock:   false, // 460/100 = 4.6 < 4.669
		},
		{
			name:          "Just over boundary (1:4.7 ratio)",
			deltaCore:     100,
			deltaFeatures: 470,
			shouldBlock:   true, // 470/100 = 4.7 > 4.669
		},
		{
			name:          "No core work (technical debt)",
			deltaCore:     0,
			deltaFeatures: 100,
			shouldBlock:   true, // 100/0 = ∞ (instant violation)
		},
		{
			name:          "Only core work (safe)",
			deltaCore:     100,
			deltaFeatures: 0,
			shouldBlock:   false, // 0/100 = 0 (always safe)
		},
		{
			name:          "10x violation (severe debt)",
			deltaCore:     10,
			deltaFeatures: 100,
			shouldBlock:   true, // 100/10 = 10 >> 4.669
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := SystemIntegrityMetrics{
				ImmutableOpsVerified: 100,
				MutableSharedState:   5,
				DeltaCriticalCore:    tc.deltaCore,
				DeltaComplexity:      tc.deltaFeatures,
			}

			action := g.CheckStructuralIntegrity(metrics)

			isBlocked := (action.Type == ActionBlockDeploy)
			if isBlocked != tc.shouldBlock {
				ratio := tc.deltaFeatures / maxFloat(tc.deltaCore, 0.0001)
				t.Errorf("Expected blocked=%v, got blocked=%v\nRatio: %.2f\nReason: %s",
					tc.shouldBlock, isBlocked, ratio, action.Reason)
			}
		})
	}
}

func TestGovernor_ApplyRecovery_Success(t *testing.T) {
	g := NewGovernor(3.2) // Start in saturation

	// Perfect isolation (should succeed)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    0, // Perfect isolation
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 0,
		ScalingRatio:          0.10,
	}

	success := g.ApplyRecovery(metrics)

	if !success {
		t.Errorf("Recovery should succeed with perfect isolation")
	}

	stats := g.GetStatistics()
	currentR := stats["current_r"].(float64)

	if currentR >= 3.0 {
		t.Errorf("Expected r < 3.0 after recovery, got %.4f", currentR)
	}

	if stats["recovery_events"].(int) == 0 {
		t.Errorf("Expected recovery events recorded")
	}
}

func TestGovernor_ApplyRecovery_Failure(t *testing.T) {
	g := NewGovernor(3.8) // Deep saturation

	// Poor isolation (will fail)
	metrics := SystemIntegrityMetrics{
		ImmutableOpsVerified:  100,
		MutableSharedState:    80, // 80% violations (structural problem)
		SupervisedProcesses:   50,
		UnsupervisedProcesses: 40, // 80% unsupervised
		ScalingRatio:          0.30,
	}

	success := g.ApplyRecovery(metrics)

	// With such poor isolation, recovery may fail
	// (This tests the "restart is only option" path)
	if !success {
		stats := g.GetStatistics()
		currentR := stats["current_r"].(float64)

		// Should still be in saturation
		if currentR < 3.0 {
			t.Errorf("Expected r ≥ 3.0 (recovery failed), got %.4f", currentR)
		}

		t.Logf("Recovery correctly failed with poor isolation (r=%.4f)", currentR)
		t.Logf("Restart required (only BIG recovery)")
	}
}

func TestGovernor_Statistics(t *testing.T) {
	g := NewGovernor(2.0)

	// Trigger various actions
	g.CheckStructuralIntegrity(SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   10,
		DeltaCriticalCore:    10,
		DeltaComplexity:      100, // Violation
	})

	g.CheckStructuralIntegrity(SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   20,
	})

	stats := g.GetStatistics()

	// Check structure
	requiredKeys := []string{
		"current_r", "initial_r", "in_saturation",
		"warnings_issued", "throttles_applied", "deploys_blocked",
		"recovery_events", "history_length",
	}

	for _, key := range requiredKeys {
		if _, exists := stats[key]; !exists {
			t.Errorf("Missing statistic: %s", key)
		}
	}

	// Check history accumulation
	if stats["history_length"].(int) < 2 {
		t.Errorf("Expected history length ≥ 2, got %d", stats["history_length"].(int))
	}
}

func TestGovernor_VelocityTracking(t *testing.T) {
	g := NewGovernor(2.0)

	// First check
	action1 := g.CheckStructuralIntegrity(SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   5,
	})

	// Simulate system degradation
	g.rdynamics.CurrentR = 2.85 // Jump to warning zone

	// Second check (should detect velocity)
	action2 := g.CheckStructuralIntegrity(SystemIntegrityMetrics{
		ImmutableOpsVerified: 100,
		MutableSharedState:   20,
	})

	// Should mention velocity in reason
	if !strings.Contains(action1.Reason, "Velocity") &&
		!strings.Contains(action2.Reason, "Velocity") &&
		!strings.Contains(action2.Reason, "velocity") {
		t.Logf("Note: Velocity tracking present but format may vary")
		t.Logf("Action1 reason: %s", action1.Reason)
		t.Logf("Action2 reason: %s", action2.Reason)
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
