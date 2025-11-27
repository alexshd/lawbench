package lawbench

import (
	"fmt"
	"time"
)

// Governor implements adaptive load control based on the coupling parameter (r).
// It monitors r(t) and applies corrective action when the system approaches
// or enters the saturation region.
//
// Control loop:
// - Continuous monitoring of Δr/Δt (rate of change)
// - Small corrections when approaching saturation (gradual throttling)
// - Aggressive shedding when at saturation point (emergency throttling)
// - Rejection when capacity limits violated (block deployment)
type Governor struct {
	// Monitoring state
	rdynamics     *RDynamics
	lastCheck     time.Time
	checkInterval time.Duration

	// Thresholds
	warningThreshold    float64 // r > 2.8 → warning
	dangerThreshold     float64 // r > 2.9 → danger
	saturationThreshold float64 // r ≥ 3.0 → saturation point

	// Hysteresis (prevents bang-bang oscillation)
	inThrottleMode        bool          // Currently applying aggressive throttling
	throttleEnteredAt     time.Time     // When throttling was applied
	throttleMinDuration   time.Duration // Minimum time to stay in throttle mode
	throttleExitThreshold float64       // r must drop below this to exit throttle (2.0)

	// Action history
	warnings       int
	throttleEvents int
	deployBlocked  int
}

// ActionType represents the governor's decision.
type ActionType string

const (
	ActionStable      ActionType = "STABLE"       // System healthy, no action
	ActionWarning     ActionType = "WARNING"      // Approaching saturation, monitor closely
	ActionPacing      ActionType = "PACING"       // Apply small correction (shed 10-20% load)
	ActionThrottle    ActionType = "THROTTLE"     // Emergency correction (shed 50%+ load)
	ActionBlockDeploy ActionType = "BLOCK_DEPLOY" // Reject change (violates capacity limits)
	ActionRestart     ActionType = "RESTART"      // Only option if throttling fails
)

// Action represents the governor's decision and reasoning.
type Action struct {
	Type       ActionType
	Reason     string
	Mitigation string
	Metrics    SystemIntegrityMetrics
	Timestamp  time.Time
}

// NewGovernor creates a system governor with standard thresholds.
func NewGovernor(initialR float64) *Governor {
	return &Governor{
		rdynamics: &RDynamics{
			InitialR:    initialR,
			CurrentR:    initialR,
			TargetR:     2.4, // Target 80% of saturation
			History:     []float64{initialR},
			InSaturationZone: initialR >= 3.0,
		},
		lastCheck:           time.Now(),
		checkInterval:       time.Second, // Check every second
		warningThreshold:    2.8,
		dangerThreshold:     2.9,
		saturationThreshold: 3.0,

		// Hysteresis: prevent oscillation
		inThrottleMode:        false,
		throttleMinDuration:   60 * time.Second, // Stay in throttle for at least 1 minute
		throttleExitThreshold: 2.0,              // Must drop to 2.0 to exit (not just <3.0)
	}
}

// CheckStructuralIntegrity is the main decision function.
// This is what gets called on every request, deployment, or periodic check.
//
// The "Control Loop": Monitor → Decide → Act
func (g *Governor) CheckStructuralIntegrity(metrics SystemIntegrityMetrics) Action {
	now := time.Now()

	// Calculate current r from metrics
	currentR := CalculateSystemDNA(metrics)
	g.rdynamics.CurrentR = currentR
	g.rdynamics.History = append(g.rdynamics.History, currentR)
	g.rdynamics.InSaturationZone = currentR >= g.saturationThreshold

	// Calculate Δr/Δt (rate of change)
	var velocity float64
	if len(g.rdynamics.History) > 1 {
		deltaR := g.rdynamics.History[len(g.rdynamics.History)-1] -
			g.rdynamics.History[len(g.rdynamics.History)-2]
		deltaT := now.Sub(g.lastCheck).Seconds()
		if deltaT > 0 {
			velocity = deltaR / deltaT
		}
	}
	g.lastCheck = now

	// Helper for max float
	maxFloat := func(a, b float64) float64 {
		if a > b {
			return a
		}
		return b
	} // ========================================
	// Phase I: Check Deployment Constraint
	// ========================================
	// The "21% Rule" (1/δ ≈ 0.214)
	// Mathematical Proof of Technical Debt

	// Check if this is a deployment (any delta values provided)
	if metrics.DeltaCriticalCore > 0 || metrics.DeltaComplexity > 0 {
		// Special case: no core work but adding complexity = instant violation
		if metrics.DeltaCriticalCore == 0 && metrics.DeltaComplexity > 0 {
			g.deployBlocked++
			return Action{
				Type: ActionBlockDeploy,
				Reason: fmt.Sprintf(
					"Σ_R Violation: Pure Technical Debt Accumulation\n"+
						"  ΔComplexity (Tier 2/3): %.0f LOC\n"+
						"  ΔCore (Tier 1): 0 LOC\n"+
						"  Ratio: ∞ (undefined)\n"+
						"  This is 100%% Technical Debt.\n"+
						"  Current r: %.4f",
					metrics.DeltaComplexity, currentR,
				),
				Mitigation: "REQUIRED:\n" +
					"  Must refactor Tier 1 Core before adding Tier 2/3 complexity\n" +
					"  Cannot build features without strengthening foundation\n" +
					"  Technical Debt Formula: debt = ΔComplexity (when ΔCore = 0)",
				Metrics:   metrics,
				Timestamp: now,
			}
		}

		growthRatio := metrics.DeltaComplexity / metrics.DeltaCriticalCore
		maxRatio := FeigenbaumDelta // ≈ 4.669

		if growthRatio > maxRatio {
			g.deployBlocked++
			return Action{
				Type: ActionBlockDeploy,
				Reason: fmt.Sprintf(
					"Σ_R Violation: Complexity Growth Ratio %.2f exceeds Feigenbaum Limit %.2f\n"+
						"  ΔComplexity (Tier 2/3): %.0f LOC\n"+
						"  ΔCore (Tier 1): %.0f LOC\n"+
						"  Ratio: %.2f > %.2f (4.669x)\n"+
						"  This is Technical Debt accumulation.\n"+
						"  Current r: %.4f (approaching saturation at 3.0)",
					growthRatio, maxRatio,
					metrics.DeltaComplexity, metrics.DeltaCriticalCore,
					growthRatio, maxRatio, currentR,
				),
				Mitigation: "OPTIONS:\n" +
					"  1. Refactor Tier 1 Core (increase denominator)\n" +
					"  2. Reduce Tier 2/3 Features (decrease numerator)\n" +
					"  3. Split into separate systems (reduce coupling)\n" +
					"\nTechnical Debt Formula: debt = ΔComplexity - (ΔCore × 4.669)",
				Metrics:   metrics,
				Timestamp: now,
			}
		}
	}

	// ========================================
	// Phase II: Check Runtime State (r value)
	// ========================================

	// SATURATION ZONE: r ≥ 3.0
	// WITH HYSTERESIS: Once in throttle mode, stay there until conditions improve
	if currentR >= g.saturationThreshold || g.inThrottleMode {
		// Check if we can exit throttle mode (hysteresis)
		if g.inThrottleMode {
			timeSinceThrottle := now.Sub(g.throttleEnteredAt)

			// Exit conditions:
			// 1. Minimum time elapsed (prevent rapid cycling)
			// 2. r dropped significantly below threshold (not just <3.0)
			if timeSinceThrottle >= g.throttleMinDuration && currentR < g.throttleExitThreshold {
				g.inThrottleMode = false
				// Fall through to normal state checking below
			} else {
				// Still in throttle mode (hysteresis active)
				return Action{
					Type: ActionThrottle,
					Reason: fmt.Sprintf(
						"THROTTLE MODE (Hysteresis): r=%.4f\n"+
							"  Time throttled: %.0f seconds\n"+
							"  Need: %.0f more seconds OR r < %.1f\n"+
							"  Current: r=%.4f (must stabilize below %.1f)\n"+
							"  Hysteresis prevents rapid throttle cycling",
						currentR,
						timeSinceThrottle.Seconds(),
						(g.throttleMinDuration - timeSinceThrottle).Seconds(),
						g.throttleExitThreshold,
						currentR, g.throttleExitThreshold,
					),
					Mitigation: "ONGOING THROTTLE:\n" +
						"  Maintaining 50-70%% load shed\n" +
						"  Waiting for system to stabilize\n" +
						"  Hysteresis prevents oscillation",
					Metrics:   metrics,
					Timestamp: now,
				}
			}
		}

		// Enter throttle mode (or already in it)
		if !g.inThrottleMode {
			g.inThrottleMode = true
			g.throttleEnteredAt = now
			g.throttleEvents++
		}

		// Calculate how deep into saturation
		saturationDepth := currentR - g.saturationThreshold

		return Action{
			Type: ActionThrottle,
			Reason: fmt.Sprintf(
				"SATURATION DETECTED: r=%.4f ≥ 3.0 (boundary)\n"+
					"  Saturation depth: %.4f\n"+
					"  System entered period-doubling cascade\n"+
					"  Behavior is unpredictable\n"+
					"  Throughput will collapse if uncorrected\n"+
					"  Recovery required: %d iterations needed",
				currentR, saturationDepth, estimateRecoveryIterations(saturationDepth),
			),
			Mitigation: "IMMEDIATE ACTIONS:\n" +
				"  1. THROTTLE: Shed 50-70%% of traffic immediately\n" +
				"  2. Apply recovery (enforce Law I: Isolation)\n" +
				"  3. Monitor r(t) until r < 3.0\n" +
				"  4. If fails after 20 iterations → RESTART required\n" +
				"\nRoot Cause Analysis:\n" +
				fmt.Sprintf("  Isolation ratio: %.2f (mutable/immutable)\n",
					float64(metrics.MutableSharedState)/float64(max(metrics.ImmutableOpsVerified, 1))) +
				fmt.Sprintf("  Supervision ratio: %.2f (unsupervised/supervised)\n",
					float64(metrics.UnsupervisedProcesses)/float64(max(metrics.SupervisedProcesses, 1))) +
				fmt.Sprintf("  Scaling ratio: %.4f (should be ≤ 0.214)\n", metrics.ScalingRatio),
			Metrics:   metrics,
			Timestamp: now,
		}
	}

	// DANGER ZONE: 2.9 < r < 3.0
	if currentR >= g.dangerThreshold {
		return Action{
			Type: ActionPacing,
			Reason: fmt.Sprintf(
				"DANGER: r=%.4f approaching saturation boundary (3.0)\n"+
					"  Distance to saturation: %.4f\n"+
					"  Velocity (Δr/Δt): %.6f per second\n"+
					"  Time to saturation: %.1f seconds (if velocity constant)\n"+
					"  Applying preventive correction (incremental correction)",
				currentR, g.saturationThreshold-currentR, velocity,
				(g.saturationThreshold-currentR)/maxFloat(velocity, 0.001),
			),
			Mitigation: "PREVENTIVE ACTIONS:\n" +
				"  1. PACING: Shed 20%% of traffic (gentle correction)\n" +
				"  2. Apply Feigenbaum governance (limit scaling)\n" +
				"  3. Increase monitoring frequency (10x)\n" +
				"  4. Alert on-call engineer\n" +
				"\nPreventive Formula: correction = (r - 2.9) × 0.5",
			Metrics:   metrics,
			Timestamp: now,
		}
	}

	// WARNING ZONE: 2.8 < r < 2.9
	if currentR >= g.warningThreshold {
		g.warnings++
		return Action{
			Type: ActionWarning,
			Reason: fmt.Sprintf(
				"WARNING: r=%.4f above optimal (2.8)\n"+
					"  Operating in warning zone\n"+
					"  Velocity: %.6f per second\n"+
					"  Margin to saturation: %.4f\n"+
					"  Monitor closely for escalation",
				currentR, velocity, g.saturationThreshold-currentR,
			),
			Mitigation: "MONITORING ACTIONS:\n" +
				"  1. Watch Δr/Δt (rate of change)\n" +
				"  2. Identify coupling sources (Law I violations?)\n" +
				"  3. Prepare for pacing if r > 2.9\n" +
				"  4. Review recent deployments\n" +
				"\nTarget: Return to r ≤ 2.8 (optimal operating point)",
			Metrics:   metrics,
			Timestamp: now,
		}
	}

	// STABLE ZONE: r < 2.8
	return Action{
		Type: ActionStable,
		Reason: fmt.Sprintf(
			"STABLE: r=%.4f (healthy)\n"+
				"  Velocity: %.6f per second\n"+
				"  Margin to saturation: %.4f\n"+
				"  System operating in stable equilibrium",
			currentR, velocity, g.saturationThreshold-currentR,
		),
		Mitigation: "No action required. Continue monitoring.",
		Metrics:    metrics,
		Timestamp:  now,
	}
}

// ApplyRecovery executes iterative correction until stable.
// Returns true if successful, false if restart required.
func (g *Governor) ApplyRecovery(metrics SystemIntegrityMetrics) bool {
	const maxIterations = 20

	finalR, iterations := g.rdynamics.ApplyRecoveryUntilStable(metrics, maxIterations)

	// If still in saturation after max iterations, restart is the only option
	if finalR >= g.saturationThreshold {
		return false // Recovery failed
	}

	g.throttleEvents += iterations
	return true // Success
}

// GetStatistics returns governor operational stats.
func (g *Governor) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"current_r":             g.rdynamics.CurrentR,
		"initial_r":             g.rdynamics.InitialR,
		"in_saturation":              g.rdynamics.InSaturationZone,
		"warnings_issued":       g.warnings,
		"throttles_applied":        g.throttleEvents,
		"deploys_blocked":       g.deployBlocked,
		"recovery_events": g.rdynamics.RecoveryEvents,
		"history_length":        len(g.rdynamics.History),
	}
}

// estimateRecoveryIterations predicts iterations needed based on saturation depth.
func estimateRecoveryIterations(saturationDepth float64) int {
	// Each iteration can correct at most 1/δ ≈ 0.214
	// With 50% efficiency: 0.214 × 0.5 ≈ 0.107 per iteration
	iterationsNeeded := int(saturationDepth / 0.107)
	if iterationsNeeded < 1 {
		iterationsNeeded = 1
	}
	return iterationsNeeded
}
