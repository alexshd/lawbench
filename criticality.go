package lawbench

import (
	"fmt"
	"math"
)

// Feigenbaum constant: δ ≈ 4.669201609...
// Universal scaling factor governing the rate of structural decay toward instability.
//
// NOTE: Precision limited to 4 decimal places for distributed systems.
// Network I/O noise floor (~ms) makes sub-millisecond precision meaningless.
// Value rounded from 4.669201609102990671853203820466 (theoretical).
const FeigenbaumDelta = 4.6692

// CriticalityScalingRatio is the inverse of Feigenbaum delta: 1/δ ≈ 0.214
// Maximum permissible ratio of complexity added to extensible layers
// relative to changes in critical core components.
const CriticalityScalingRatio = 1.0 / FeigenbaumDelta // ≈ 0.214 (21.4%)

// SystemDNAConstraint defines the stable equilibrium range for coupling parameter r.
// The logistic map equation: x_next = r·x·(1-x)
// Stable equilibrium requires: 1 < r < 3
type SystemDNAConstraint struct {
	MinR float64 // Minimum coupling (r > 1 for non-trivial dynamics)
	MaxR float64 // Maximum coupling (r < 3 to avoid bifurcation cascade)
}

// StableDNAConstraint returns the mathematically proven stable range.
var StableDNAConstraint = SystemDNAConstraint{
	MinR: 1.0, // Below this: system converges to 0 (trivial death)
	MaxR: 3.0, // Above this: period-doubling cascade begins
}

// CriticalityScalingConstraint enforces the Feigenbaum scaling law.
// Ensures that complexity growth respects the universal rate constant.
//
// Mathematical formulation:
//
//	Δ Complexity (Tier 2/3) / Δ Critical Core (Tier 1) ≤ 1/δ
//
// Where:
//   - Tier 1 (Critical Core): Immutable state, verified by Abstract Algebra (Law I)
//   - Tier 2/3 (Extensible): High-churn components, supervised for failure (Law II)
//   - δ ≈ 4.669: Universal constant from bifurcation theory
//   - 1/δ ≈ 0.214: Maximum safe scaling ratio (21.4%)
type CriticalityScalingConstraint struct {
	DeltaCriticalCore float64 // Changes to Tier 1 (lines, complexity, API surface)
	DeltaComplexity   float64 // Changes to Tier 2/3 (extensible layers)
	MaxRatio          float64 // Maximum allowed ratio (default: 1/δ)
	CurrentCouplingR  float64 // Current system coupling parameter
	TargetCouplingR   float64 // Desired coupling parameter (< 3.0)
}

// NewCriticalityConstraint creates a constraint with Feigenbaum scaling law.
func NewCriticalityConstraint(deltaCritical, deltaComplex float64) CriticalityScalingConstraint {
	return CriticalityScalingConstraint{
		DeltaCriticalCore: deltaCritical,
		DeltaComplexity:   deltaComplex,
		MaxRatio:          CriticalityScalingRatio,
		CurrentCouplingR:  0.0,                      // Unknown initially
		TargetCouplingR:   StableDNAConstraint.MaxR, // Default: stay below 3.0
	}
}

// Validate checks if the scaling respects the Feigenbaum constraint.
// Returns error if ratio exceeds 1/δ ≈ 0.214.
func (c CriticalityScalingConstraint) Validate() error {
	if c.DeltaCriticalCore == 0 {
		return fmt.Errorf("zero critical core changes: cannot divide by zero")
	}

	ratio := c.DeltaComplexity / c.DeltaCriticalCore

	if ratio > c.MaxRatio {
		return fmt.Errorf(
			"criticality scaling violation: ratio %.4f exceeds Feigenbaum limit %.4f (1/δ)\n"+
				"  ΔComplexity (Tier 2/3): %.2f\n"+
				"  ΔCritical Core (Tier 1): %.2f\n"+
				"  Ratio: %.4f > %.4f\n"+
				"  Risk: Accelerating toward instability threshold (r → 3.0)\n"+
				"  Action: Reduce extensible complexity or strengthen critical core",
			ratio, c.MaxRatio,
			c.DeltaComplexity, c.DeltaCriticalCore,
			ratio, c.MaxRatio,
		)
	}

	return nil
}

// Ratio returns the current complexity-to-core ratio.
func (c CriticalityScalingConstraint) Ratio() float64 {
	if c.DeltaCriticalCore == 0 {
		return math.Inf(1) // Infinite ratio (violation)
	}
	return c.DeltaComplexity / c.DeltaCriticalCore
}

// Headroom returns how much more complexity can be added before hitting the limit.
func (c CriticalityScalingConstraint) Headroom() float64 {
	maxAllowed := c.DeltaCriticalCore * c.MaxRatio
	return maxAllowed - c.DeltaComplexity
}

// IsStableEquilibrium checks if coupling parameter r is in stable DNA range.
func (c CriticalityScalingConstraint) IsStableEquilibrium() bool {
	return c.CurrentCouplingR > StableDNAConstraint.MinR &&
		c.CurrentCouplingR < StableDNAConstraint.MaxR
}

// DistanceToInstabilityBoundary returns how close the system is to bifurcation cascade.
// Returns negative if already in unstable region (r ≥ 3.0).
func (c CriticalityScalingConstraint) DistanceToInstabilityBoundary() float64 {
	return StableDNAConstraint.MaxR - c.CurrentCouplingR
}

// PredictCouplingImpact estimates how adding complexity affects coupling parameter r.
// This is a heuristic model: r increases proportionally to complexity ratio.
func (c CriticalityScalingConstraint) PredictCouplingImpact() float64 {
	if c.CurrentCouplingR == 0 {
		return 0 // Unknown baseline
	}

	// Model: Each 1.0 increase in ratio adds (1/δ) to coupling parameter
	// This reflects that complexity growth accelerates interdependence.
	ratioIncrease := c.Ratio()
	couplingIncrease := ratioIncrease * CriticalityScalingRatio

	return c.CurrentCouplingR + couplingIncrease
}

// SystemIntegrityMetrics captures the three-law enforcement status.
type SystemIntegrityMetrics struct {
	// Law I: Isolation (Abstract Algebra)
	ImmutableOpsVerified int     // Number of operations proven immutable
	MutableSharedState   int     // Number of violations detected
	IsolationScore       float64 // 1.0 = perfect isolation

	// Law II: Supervision (Erlang-style)
	SupervisedProcesses   int     // Processes under supervision tree
	UnsupervisedProcesses int     // Processes without supervision
	MeanTimeToRestart     float64 // Average restart time (lower = better)

	// Law III: Criticality Scaling (Feigenbaum)
	CriticalCoreLOC      int     // Tier 1 lines of code
	ExtensibleLOC        int     // Tier 2/3 lines of code
	ScalingRatio         float64 // Extensible/Critical ratio
	FeigenbaumCompliance bool    // True if ratio ≤ 1/δ

	// Deployment deltas (for Σ_R constraint checking)
	DeltaCriticalCore float64 // Change in Tier 1 (LOC, API surface)
	DeltaComplexity   float64 // Change in Tier 2/3 (LOC, dependencies)

	// Derived: System DNA (coupling parameter r)
	EstimatedCoupling     float64 // Current r value
	InstabilityBoundaryDistance float64 // Distance to r = 3.0
	StableEquilibrium     bool    // True if 1 < r < 3
}

// CalculateSystemDNA derives the coupling parameter r from metrics.
// This is a model that combines all three laws into a single r estimate.
func CalculateSystemDNA(metrics SystemIntegrityMetrics) float64 {
	// Base coupling from isolation violations (Law I)
	isolationPenalty := float64(metrics.MutableSharedState) /
		float64(max(metrics.ImmutableOpsVerified, 1))

	// Supervision penalty (Law II)
	supervisionPenalty := float64(metrics.UnsupervisedProcesses) /
		float64(max(metrics.SupervisedProcesses, 1))

	// Scaling penalty (Law III)
	scalingPenalty := metrics.ScalingRatio / CriticalityScalingRatio

	// Model: r starts at 1.0 (minimum), increases with violations
	// Each penalty can add up to 1.0, so worst case r ≈ 4.0 (deep instability)
	r := 1.0 + isolationPenalty + supervisionPenalty + scalingPenalty

	return r
}

// ValidateSystemDNA checks if metrics satisfy all three laws.
func ValidateSystemDNA(metrics SystemIntegrityMetrics) error {
	r := CalculateSystemDNA(metrics)

	if r < StableDNAConstraint.MinR {
		return fmt.Errorf("system coupling too low: r=%.4f < %.1f (trivial dynamics)",
			r, StableDNAConstraint.MinR)
	}

	if r >= StableDNAConstraint.MaxR {
		return fmt.Errorf("system coupling in unstable region: r=%.4f ≥ %.1f\n"+
			"  Isolation violations: %d\n"+
			"  Unsupervised processes: %d\n"+
			"  Scaling ratio: %.4f (limit: %.4f)\n"+
			"  Action: Enforce Law I (Isolation), Law II (Supervision), Law III (Scaling)",
			r, StableDNAConstraint.MaxR,
			metrics.MutableSharedState,
			metrics.UnsupervisedProcesses,
			metrics.ScalingRatio, CriticalityScalingRatio,
		)
	}

	return nil
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RDynamics tracks the evolution of coupling parameter r over time.
type RDynamics struct {
	InitialR             float64   // Starting coupling parameter
	CurrentR             float64   // Current coupling parameter
	TargetR              float64   // Desired stable r (< 3.0)
	History              []float64 // Historical r values
	RecoveryEvents int       // Count of corrections applied
	InSaturationZone          bool      // True if r ≥ 3.0
}

// NewRDynamics creates r dynamics tracker with initial state.
func NewRDynamics(initialR float64) RDynamics {
	// At r = 3.0, system is AT instability threshold (fixed point loses stability)
	// We treat r >= 3.0 as unstable region
	inInstability := initialR >= StableDNAConstraint.MaxR
	return RDynamics{
		InitialR:             initialR,
		CurrentR:             initialR,
		TargetR:              StableDNAConstraint.MaxR * 0.8, // Target 80% of limit (r ≈ 2.4)
		History:              []float64{initialR},
		RecoveryEvents: 0,
		InSaturationZone:          inInstability,
	}
}

// ApplyRecovery corrects r by enforcing Law I (Isolation).
// This is INCREMENTAL correction: small adjustments, not large disruptions.
//
// Like a adaptive controller applying gentle pacing (gradual reduction):
// - Small correction per pulse (based on Feigenbaum δ)
// - Multiple iterations to reach attractor
// - Large disruptions = panic() = system-wide destabilization
//
// Correction per pulse governed by δ (Feigenbaum constant):
// - Maximum safe correction = 1/δ ≈ 0.214 per iteration
// - Prevents throttling from being worse than the instability
//
// Returns the new r value after ONE small correction pulse.
func (rd *RDynamics) ApplyRecovery(metrics SystemIntegrityMetrics) float64 {
	if !rd.InSaturationZone {
		return rd.CurrentR // No correction needed
	}

	// Calculate isolation quality (Law I compliance)
	isolationRatio := float64(metrics.MutableSharedState) /
		float64(max(metrics.ImmutableOpsVerified, 1))

	// How far into instability we are
	instabilityDepth := rd.CurrentR - StableDNAConstraint.MaxR

	// Correction strength based on isolation quality
	// Perfect isolation (ratio = 0) → correction_factor = 1.0
	// Poor isolation (ratio = 1) → correction_factor = 0.5
	// No isolation (ratio >> 1) → correction_factor ≈ 0
	correctionFactor := 1.0 / (1.0 + isolationRatio)

	// CRITICAL: Correction pulse limited by 1/δ (Feigenbaum constraint)
	// This is the maximum safe change per iteration
	// Larger corrections = panic() effect (destabilize all connected nodes)
	maxSafePulse := CriticalityScalingRatio // 1/δ ≈ 0.214

	// Actual pulse: smaller of (what's needed based on isolation, or safe limit)
	// With perfect isolation, use 50% of depth per iteration (but capped by 1/δ)
	desiredPulse := instabilityDepth * correctionFactor * 0.5 // 50% of depth
	correctionPulse := math.Min(desiredPulse, maxSafePulse)

	// Apply small incremental correction
	newR := rd.CurrentR - correctionPulse

	// If we're exactly at boundary (r = 3.0), apply one more small pulse
	// to ensure we're safely below (like incremental correction: one more beat)
	if math.Abs(newR-StableDNAConstraint.MaxR) < 0.0001 {
		newR = StableDNAConstraint.MaxR * 0.999 // 0.1% below boundary
	}

	// Enforce bounds
	if newR < StableDNAConstraint.MinR {
		newR = StableDNAConstraint.MinR
	}

	rd.CurrentR = newR
	rd.History = append(rd.History, newR)
	rd.RecoveryEvents++
	rd.InSaturationZone = newR >= StableDNAConstraint.MaxR

	return newR
}

// ApplyRecoveryUntilStable applies iterative small corrections until r < 3.0.
// Like incremental correction: multiple gentle pulses, not one large disruption.
//
// Each pulse limited by 1/δ to prevent panic() cascade.
// Returns: (final_r, iterations_needed)
func (rd *RDynamics) ApplyRecoveryUntilStable(metrics SystemIntegrityMetrics, maxIterations int) (float64, int) {
	iterations := 0

	for rd.InSaturationZone && iterations < maxIterations {
		rd.ApplyRecovery(metrics)
		iterations++
	}

	return rd.CurrentR, iterations
}

// ApplyFeigenbaumGovernance prevents r from growing due to scaling.
// This is the preventive constraint: ensure Δr < 1/δ threshold.
//
// Mathematical model:
//
//	r_next = r_current + (scalingRatio / δ)
//
// If scalingRatio ≤ 1/δ, then Δr is bounded and r stays stable.
// If scalingRatio > 1/δ, then Δr accelerates and r → instability.
func (rd *RDynamics) ApplyFeigenbaumGovernance(scalingRatio float64) float64 {
	// Calculate r increment from scaling
	// Model: Each unit of scaling ratio adds (1/δ²) to r
	// This reflects that complexity growth accelerates coupling nonlinearly
	rIncrement := scalingRatio * (1.0 / (FeigenbaumDelta * FeigenbaumDelta))

	// Apply increment
	newR := rd.CurrentR + rIncrement

	// Update state
	rd.CurrentR = newR
	rd.History = append(rd.History, newR)
	rd.InSaturationZone = newR >= StableDNAConstraint.MaxR

	return newR
}

// CorrectRAfterRecovery combines both mechanisms:
// 1. Recovery (active correction via Law I)
// 2. Feigenbaum governance (preventive constraint via Law III)
//
// This is the complete r management strategy:
//   - If r ≥ 3.0: Apply recovery (Law I enforcement)
//   - Always: Apply Feigenbaum governance to prevent future escalation
func CorrectRAfterRecovery(rd *RDynamics, metrics SystemIntegrityMetrics, scalingRatio float64) float64 {
	// Phase 1: Recovery (if needed)
	if rd.InSaturationZone {
		rd.ApplyRecovery(metrics)
	}

	// Phase 2: Feigenbaum governance (always)
	rd.ApplyFeigenbaumGovernance(scalingRatio)

	return rd.CurrentR
}

// PerpetuaStructuralIntegrity verifies Σ_R constraint.
// This is the unified law: r must stay in [1, 3) through combined enforcement.
//
// Mathematical formulation:
//
//	Σ_R ≡ Enforce { 1 < r_eff(x, ΔC) < 3 } via { ΔComplexity/ΔCore ≤ 1/δ }
func PerpetualStructuralIntegrity(rd *RDynamics, metrics SystemIntegrityMetrics) error {
	// Check DNA constraint
	if rd.CurrentR < StableDNAConstraint.MinR {
		return fmt.Errorf("Σ_R violation: r=%.4f < %.1f (system trivial/dead)",
			rd.CurrentR, StableDNAConstraint.MinR)
	}

	if rd.CurrentR >= StableDNAConstraint.MaxR {
		return fmt.Errorf("Σ_R violation: r=%.4f ≥ %.1f (unstable region)\n"+
			"  Recovery required: Enforce Law I (Isolation)\n"+
			"  Current isolation ratio: %.4f (mutable/immutable)\n"+
			"  Target: Reduce mutable state to achieve r < 3.0",
			rd.CurrentR, StableDNAConstraint.MaxR,
			float64(metrics.MutableSharedState)/float64(max(metrics.ImmutableOpsVerified, 1)))
	}

	// Check Feigenbaum constraint
	scalingRatio := metrics.ScalingRatio
	if scalingRatio > CriticalityScalingRatio {
		return fmt.Errorf("Σ_R violation: scaling ratio %.4f > %.4f (1/δ)\n"+
			"  Risk: r will increase toward instability threshold\n"+
			"  Current r: %.4f\n"+
			"  Predicted r (if unchecked): %.4f\n"+
			"  Action: Reduce complexity growth or strengthen critical core",
			scalingRatio, CriticalityScalingRatio,
			rd.CurrentR,
			rd.CurrentR+scalingRatio*(1.0/(FeigenbaumDelta*FeigenbaumDelta)))
	}

	return nil
}

// RTrajectory simulates r evolution over time given a sequence of events.
type RTrajectory struct {
	Events []REvent  // Sequence of system events
	R      []float64 // r value after each event
}

// REvent represents a system change that affects coupling parameter.
type REvent struct {
	Type         string                 // "scaling", "recovery", "violation"
	ScalingRatio float64                // For scaling events
	Metrics      SystemIntegrityMetrics // For recovery
	Description  string                 // Human-readable description
}

// SimulateRTrajectory models how r evolves under a sequence of architectural decisions.
// This is the predictive tool: "What happens to r if we add this feature?"
func SimulateRTrajectory(initialR float64, events []REvent) RTrajectory {
	rd := NewRDynamics(initialR)
	trajectory := RTrajectory{
		Events: events,
		R:      []float64{initialR},
	}

	for _, event := range events {
		switch event.Type {
		case "scaling":
			// Apply Feigenbaum governance
			rd.ApplyFeigenbaumGovernance(event.ScalingRatio)

		case "recovery":
			// Apply active correction
			rd.ApplyRecovery(event.Metrics)

		case "violation":
			// Isolation violation increases r directly
			violationPenalty := float64(event.Metrics.MutableSharedState) /
				float64(max(event.Metrics.ImmutableOpsVerified, 1))
			rd.CurrentR += violationPenalty
			rd.InSaturationZone = rd.CurrentR >= StableDNAConstraint.MaxR
		}

		trajectory.R = append(trajectory.R, rd.CurrentR)
	}

	return trajectory
}
