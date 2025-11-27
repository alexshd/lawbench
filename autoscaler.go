package lawbench

import (
	"math"
)

// ScalingDecision represents the autoscaler's action based on r-parameter.
type ScalingDecision string

const (
	ScaleDown     ScalingDecision = "SCALE_DOWN"     // r < 1.5: System underutilized, wasting money
	Maintain      ScalingDecision = "MAINTAIN"       // 1.5 ≤ r < 2.5: The Pocket (optimal)
	ScaleUp       ScalingDecision = "SCALE_UP"       // 2.5 ≤ r < 3.0 AND N < N_peak: Add capacity
	ShedLoad      ScalingDecision = "SHED_LOAD"      // r ≥ 3.0 OR N ≥ N_peak: In retrograde zone
	EmergencyStop ScalingDecision = "EMERGENCY_STOP" // r ≥ 4.0: System in saturation, stop scaling
)

// AutoScalerMetrics contains system state for scaling decisions.
type AutoScalerMetrics struct {
	R        float64 // Current r-parameter
	CurrentN int     // Current number of nodes/workers
	Alpha    float64 // USL contention coefficient
	Beta     float64 // USL coherency coefficient
	Lambda   float64 // Serial performance (throughput at N=1)
	TargetR  float64 // Desired r value (default: 2.0)
}

// ScalingRecommendation provides detailed reasoning for the decision.
type ScalingRecommendation struct {
	Decision     ScalingDecision
	TargetN      int     // Recommended number of nodes
	Reason       string  // Human-readable explanation
	PeakN        float64 // Theoretical peak capacity point
	InRetrograde bool    // True if currently in retrograde zone
	CostSavings  float64 // Estimated cost savings (%) if scaling down
	RiskLevel    string  // LOW, MEDIUM, HIGH, CRITICAL
}

// ShouldScale determines if and how to scale based on r-parameter and USL coefficients.
//
// This is the "Billion Dollar Optimization" - it prevents the fatal mistake
// of scaling UP when you're already in the retrograde zone (N > N_peak).
//
// Traditional autoscalers (K8s HPA, AWS Auto Scaling) trigger on CPU usage:
//   - CPU > 80% → Add nodes
//   - Assumption: More nodes = Less load per node
//
// The FATAL FLAW:
//   - If β (coherency) is high, adding nodes INCREASES overhead (N²)
//   - More nodes → More crosstalk → Higher r → SATURATION
//   - You literally pay cloud provider extra money to kill your service
//
// lawbench scaling logic:
//   - r < 1.5: Scale DOWN (wasting money, system bored)
//   - 1.5 ≤ r < 2.5: MAINTAIN (The Pocket - optimal efficiency)
//   - 2.5 ≤ r < 3.0 AND N < N_peak: Scale UP (have headroom)
//   - r ≥ 3.0 OR N ≥ N_peak: SHED LOAD (retrograde zone, don't add nodes)
//
// Example:
//
//	metrics := AutoScalerMetrics{
//	    R:        2.8,
//	    CurrentN: 50,
//	    Alpha:    0.05,
//	    Beta:     0.01,
//	    Lambda:   1000,
//	    TargetR:  2.0,
//	}
//
//	rec := ShouldScale(metrics)
//	if rec.InRetrograde {
//	    // DON'T scale up, you're past peak capacity
//	    shedLoad(0.3) // Drop 30% of traffic instead
//	}
func ShouldScale(m AutoScalerMetrics) ScalingRecommendation {
	// Calculate theoretical peak capacity (where dC/dN = 0)
	// From USL: C(N) = λN / (1 + α(N-1) + βN(N-1))
	// Peak occurs at: N_peak = sqrt((1-α)/β)
	var peakN float64
	if m.Beta > 0 {
		peakN = math.Sqrt((1 - m.Alpha) / m.Beta)
	} else {
		peakN = math.Inf(1) // No coherency penalty, no peak
	}

	// Check if we're in retrograde zone
	inRetrograde := float64(m.CurrentN) >= peakN

	// Set default target r if not specified
	targetR := m.TargetR
	if targetR == 0 {
		targetR = 2.0 // The Antifragile Zone
	}

	rec := ScalingRecommendation{
		PeakN:        peakN,
		InRetrograde: inRetrograde,
	}

	// Decision tree based on r-parameter
	switch {
	case m.R >= 4.0:
		// CRITICAL: System in full saturation
		rec.Decision = EmergencyStop
		rec.TargetN = m.CurrentN // Don't change anything
		rec.Reason = "EMERGENCY: r ≥ 4.0 (full saturation). System unstable. DO NOT SCALE. " +
			"Investigate root cause immediately. Consider circuit breaker activation."
		rec.RiskLevel = "CRITICAL"

	case m.R >= 3.0:
		// System entered saturation boundary
		if inRetrograde {
			rec.Decision = ShedLoad
			rec.TargetN = int(math.Floor(peakN * 0.8)) // Scale back to 80% of peak
			rec.Reason = "SATURATION + RETROGRADE: r ≥ 3.0 AND N ≥ N_peak. " +
				"Adding nodes will INCREASE saturation (β penalty). Shed load instead."
			rec.RiskLevel = "HIGH"
		} else {
			// Still have headroom, but in saturation zone
			rec.Decision = ShedLoad
			rec.TargetN = m.CurrentN
			rec.Reason = "SATURATION: r ≥ 3.0. Shed load immediately to stabilize. " +
				"Can consider scaling up AFTER r drops below 2.5."
			rec.RiskLevel = "HIGH"
		}

	case m.R >= 2.5 && m.R < 3.0:
		// Stress detected - check if we can scale
		if inRetrograde {
			rec.Decision = ShedLoad
			rec.TargetN = m.CurrentN // Don't add nodes
			rec.Reason = "RETROGRADE: N ≥ N_peak (β dominates). " +
				"Adding nodes will increase overhead, not reduce load. Shed traffic instead."
			rec.RiskLevel = "MEDIUM"
		} else {
			// Can safely scale up
			rec.Decision = ScaleUp
			// Target: Bring r back to target
			// Rough heuristic: N_new = N_current * (r_current / r_target)
			scaleFactor := m.R / targetR
			targetN := int(math.Ceil(float64(m.CurrentN) * scaleFactor))

			// Don't exceed 80% of peak capacity (safety margin)
			maxSafeN := int(math.Floor(peakN * 0.8))
			if targetN > maxSafeN {
				targetN = maxSafeN
			}

			rec.TargetN = targetN
			rec.Reason = "STRESS: r approaching 3.0 boundary. Scale up to reduce load. " +
				"Still have headroom before retrograde zone."
			rec.RiskLevel = "MEDIUM"
		}

	case m.R >= 1.5 && m.R < 2.5:
		// The Pocket - optimal operation
		rec.Decision = Maintain
		rec.TargetN = m.CurrentN
		rec.Reason = "OPTIMAL: r in antifragile zone [1.5, 2.5]. No action needed. " +
			"System operating at peak efficiency."
		rec.RiskLevel = "LOW"

	case m.R < 1.5:
		// Underutilized - wasting money
		rec.Decision = ScaleDown
		// Target: Bring r up to target
		scaleFactor := m.R / targetR
		targetN := int(math.Floor(float64(m.CurrentN) * scaleFactor))

		// Don't scale below 1 node
		if targetN < 1 {
			targetN = 1
		}

		rec.TargetN = targetN

		// Calculate cost savings
		nodeReduction := m.CurrentN - targetN
		rec.CostSavings = (float64(nodeReduction) / float64(m.CurrentN)) * 100

		rec.Reason = "UNDERUTILIZED: r < 1.5. System bored, wasting resources. " +
			"Safe to scale down for cost savings."
		rec.RiskLevel = "LOW"
	}

	return rec
}

// CalculatePeakCapacity returns the theoretical maximum capacity point.
//
// At N_peak, adding more nodes provides NO additional throughput due to
// coherency overhead (β). Beyond this point, throughput DECREASES (retrograde).
//
// Formula: N_peak = sqrt((1-α)/β)
//
// If β = 0 (no coherency penalty), returns infinity (linear scaling forever).
func CalculatePeakCapacity(alpha, beta float64) float64 {
	if beta <= 0 {
		return math.Inf(1) // No coherency penalty, no theoretical limit
	}

	if alpha >= 1 {
		return 0 // System cannot scale at all (deadlock)
	}

	return math.Sqrt((1 - alpha) / beta)
}

// EstimateThroughput calculates expected throughput at N workers using USL.
//
// USL Formula: C(N) = λN / (1 + α(N-1) + βN(N-1))
//
// Where:
//   - λ (lambda): Serial performance (ops/sec at N=1)
//   - α (alpha): Contention coefficient (lock waiting)
//   - β (beta): Coherency coefficient (crosstalk overhead)
//   - N: Number of workers/nodes
func EstimateThroughput(N int, lambda, alpha, beta float64) float64 {
	if N <= 0 {
		return 0
	}

	numerator := lambda * float64(N)
	denominator := 1 + alpha*float64(N-1) + beta*float64(N)*float64(N-1)

	return numerator / denominator
}

// IsRetrograde checks if system is in retrograde zone (negative returns from scaling).
//
// A system is retrograde when:
//   - Current N ≥ N_peak, OR
//   - dC/dN < 0 (throughput decreases with more nodes)
//
// This is the "Death Zone" where traditional autoscalers make things WORSE.
func IsRetrograde(currentN int, alpha, beta float64) bool {
	peakN := CalculatePeakCapacity(alpha, beta)

	if math.IsInf(peakN, 1) {
		return false // No coherency penalty, can't be retrograde
	}

	return float64(currentN) >= peakN
}

// KubernetesHPATarget calculates the target replica count for K8s HPA.
//
// Use this as a custom metric adapter for Kubernetes Horizontal Pod Autoscaler:
//
//	apiVersion: autoscaling/v2
//	kind: HorizontalPodAutoscaler
//	metadata:
//	  name: myapp-hpa
//	spec:
//	  scaleTargetRef:
//	    apiVersion: apps/v1
//	    kind: Deployment
//	    name: myapp
//	  minReplicas: 2
//	  maxReplicas: 50
//	  metrics:
//	  - type: External
//	    external:
//	      metric:
//	        name: lawbench_r_value
//	      target:
//	        type: Value
//	        value: "2.0"  # Target r = 2.0 (antifragile zone)
//
// The HPA will call your custom metrics API which returns the current r value.
func KubernetesHPATarget(currentReplicas int, currentR, targetR, alpha, beta float64) int {
	metrics := AutoScalerMetrics{
		R:        currentR,
		CurrentN: currentReplicas,
		Alpha:    alpha,
		Beta:     beta,
		TargetR:  targetR,
	}

	rec := ShouldScale(metrics)

	// K8s HPA expects a target replica count
	return rec.TargetN
}
