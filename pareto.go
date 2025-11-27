package lawbench

import (
	"math"
	"sort"
	"sync"
	"time"
)

// TailDivergenceTracker detects the shift from Gaussian to Power Law (Pareto)
// by measuring the ratio between tail (P99) and median (P50).
//
// THE DOMINATED AVERAGE PROBLEM:
//
// In Gaussian distributions (stable systems):
//   - P99 ≈ 3 × Mean
//   - Outliers don't dominate
//   - Average is meaningful
//
// In Power Law distributions (saturation):
//   - P99 can be 1000 × Mean
//   - Outliers DOMINATE the average
//   - Average is a lie
//
// The shift from Gaussian → Power Law is the mathematical signature of saturation (r ≥ 3.0).
//
// Example:
//
//	tracker := NewTailDivergenceTracker(1000) // Keep last 1000 samples
//
//	// Record latencies
//	tracker.Record(5 * time.Millisecond)
//	tracker.Record(10 * time.Millisecond)
//	tracker.Record(10000 * time.Millisecond) // Black swan
//
//	// Check if system entered power law regime
//	if tracker.TailDivergenceRatio() > 10.0 {
//	    // P99 is 10x larger than P50
//	    // System shifted from Gaussian to Power Law
//	    // r likely ≥ 3.0 (saturation)
//	}
type TailDivergenceTracker struct {
	mu          sync.RWMutex
	samples     []time.Duration // Ring buffer of recent latencies
	maxSamples  int             // Buffer size
	writeIndex  int             // Next write position
	sampleCount int64           // Total samples recorded (monotonic)

	// Cached percentiles (invalidated on write)
	cachedP50  time.Duration
	cachedP99  time.Duration
	cachedP999 time.Duration
	cacheValid bool
}

// NewTailDivergenceTracker creates a tracker with a fixed-size ring buffer.
//
// The buffer size determines the time window for percentile calculation:
//   - 100 samples: Good for low-traffic systems
//   - 1000 samples: Good for medium traffic (default)
//   - 10000 samples: Good for high traffic
//
// Trade-off: Larger buffers smooth out noise but delay saturation detection.
func NewTailDivergenceTracker(maxSamples int) *TailDivergenceTracker {
	if maxSamples <= 0 {
		maxSamples = 1000 // Default
	}

	return &TailDivergenceTracker{
		samples:    make([]time.Duration, maxSamples),
		maxSamples: maxSamples,
	}
}

// Record adds a latency sample to the tracker.
//
// This is lock-free on the write path (ring buffer overwrite).
// Percentile calculation is lazy and cached until next write.
func (t *TailDivergenceTracker) Record(latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.samples[t.writeIndex] = latency
	t.writeIndex = (t.writeIndex + 1) % t.maxSamples
	t.sampleCount++
	t.cacheValid = false // Invalidate cache
}

// TailDivergenceRatio returns P99/P50 (tail divergence ratio).
//
// Interpretation:
//   - Ratio < 3:   Gaussian (stable system, r < 2.5)
//   - Ratio 3-10:  Mild skew (warning zone, 2.5 ≤ r < 3.0)
//   - Ratio > 10:  Power Law (saturation, r ≥ 3.0)
//   - Ratio > 100: Extreme tail (r ≥ 4.0, emergency)
//
// This ratio is the KEY metric for detecting saturation.
// When the tail starts to dominate, you've entered the Power Law regime.
func (t *TailDivergenceTracker) TailDivergenceRatio() float64 {
	p50 := t.P50()
	p99 := t.P99()

	if p50 == 0 {
		return 1.0 // Not enough samples
	}

	return float64(p99) / float64(p50)
}

// P50 returns the median latency (50th percentile).
func (t *TailDivergenceTracker) P50() time.Duration {
	return t.percentile(0.50)
}

// P99 returns the 99th percentile latency.
func (t *TailDivergenceTracker) P99() time.Duration {
	return t.percentile(0.99)
}

// P999 returns the 99.9th percentile latency.
func (t *TailDivergenceTracker) P999() time.Duration {
	return t.percentile(0.999)
}

// Mean returns the average latency (CAUTION: meaningless in Power Law regime).
//
// In saturation (r ≥ 3.0), the mean is dominated by outliers.
// Use TailDivergenceRatio() to check if mean is trustworthy.
func (t *TailDivergenceTracker) Mean() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.sampleCount == 0 {
		return 0
	}

	var sum int64
	effectiveSamples := t.effectiveSampleCount()

	for i := 0; i < effectiveSamples; i++ {
		sum += int64(t.samples[i])
	}

	return time.Duration(sum / int64(effectiveSamples))
}

// ParetoIndex estimates the Pareto α parameter (if distribution is Power Law).
//
// Pareto distribution: P(X > x) ≈ (x/x_min)^(-α)
//
// Interpretation:
//   - α > 2: Finite variance (mild power law)
//   - α ≤ 2: Infinite variance (extreme power law, "Black Swan" regime)
//   - α ≈ 1.16: The famous 80/20 rule (Pareto Index)
//
// If α ≤ 2, your system has INFINITE VARIANCE - saturation.
func (t *TailDivergenceTracker) ParetoIndex() float64 {
	p50 := t.P50()
	p99 := t.P99()

	if p50 == 0 || p99 == 0 {
		return 0
	}

	// Estimate α from quantile ratio
	// For Pareto: P99/P50 = (0.99/0.50)^(-1/α)
	// Solving: α = log(0.99/0.50) / log(P50/P99)

	ratio := float64(p99) / float64(p50)
	if ratio <= 1 {
		return 0 // Invalid
	}

	alpha := math.Log(0.99/0.50) / math.Log(ratio)
	return alpha
}

// IsGaussian returns true if distribution looks Gaussian (stable system).
//
// Heuristic: P99/P50 < 3 suggests Gaussian behavior.
func (t *TailDivergenceTracker) IsGaussian() bool {
	return t.TailDivergenceRatio() < 3.0
}

// IsPowerLaw returns true if distribution looks like a Power Law (saturation).
//
// Heuristic: P99/P50 > 10 suggests Power Law behavior.
func (t *TailDivergenceTracker) IsPowerLaw() bool {
	return t.TailDivergenceRatio() > 10.0
}

// EstimateR estimates the r-parameter from tail divergence.
//
// Mapping:
//   - TailRatio < 3:    r ≈ 1.5-2.0 (Gaussian, stable)
//   - TailRatio 3-10:   r ≈ 2.5-3.0 (Transitioning)
//   - TailRatio > 10:   r ≥ 3.0 (Power Law, saturation)
//   - TailRatio > 100:  r ≥ 4.0 (Extreme saturation)
//
// This is an empirical mapping. For precise r, use USL coefficients.
func (t *TailDivergenceTracker) EstimateR() float64 {
	ratio := t.TailDivergenceRatio()

	switch {
	case ratio < 3.0:
		// Gaussian regime
		return 1.5 + (ratio/3.0)*0.5 // 1.5 → 2.0

	case ratio < 10.0:
		// Transition zone
		return 2.0 + ((ratio-3.0)/7.0)*1.0 // 2.0 → 3.0

	case ratio < 100.0:
		// Power Law regime
		return 3.0 + ((ratio-10.0)/90.0)*1.0 // 3.0 → 4.0

	default:
		// Extreme saturation
		return 4.0 + math.Min((ratio-100.0)/100.0, 1.0) // 4.0 → 5.0
	}
}

// percentile calculates the p-th percentile (0 < p < 1).
func (t *TailDivergenceTracker) percentile(p float64) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	effectiveSamples := t.effectiveSampleCount()
	if effectiveSamples == 0 {
		return 0
	}

	// Copy and sort samples
	sorted := make([]time.Duration, effectiveSamples)
	copy(sorted, t.samples[:effectiveSamples])
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate index
	index := int(float64(effectiveSamples-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= effectiveSamples {
		index = effectiveSamples - 1
	}

	return sorted[index]
} // effectiveSampleCount returns the number of valid samples in the buffer.
func (t *TailDivergenceTracker) effectiveSampleCount() int {
	if t.sampleCount < int64(t.maxSamples) {
		return int(t.sampleCount)
	}
	return t.maxSamples
}

// Stats returns a comprehensive statistical snapshot.
type TailStats struct {
	SampleCount         int64
	Mean                time.Duration
	P50                 time.Duration
	P99                 time.Duration
	P999                time.Duration
	TailDivergenceRatio float64
	ParetoIndex         float64
	EstimatedR          float64
	IsGaussian          bool
	IsPowerLaw          bool
}

// GetStats returns comprehensive statistics about the distribution.
func (t *TailDivergenceTracker) GetStats() TailStats {
	return TailStats{
		SampleCount:         t.sampleCount,
		Mean:                t.Mean(),
		P50:                 t.P50(),
		P99:                 t.P99(),
		P999:                t.P999(),
		TailDivergenceRatio: t.TailDivergenceRatio(),
		ParetoIndex:         t.ParetoIndex(),
		EstimatedR:          t.EstimateR(),
		IsGaussian:          t.IsGaussian(),
		IsPowerLaw:          t.IsPowerLaw(),
	}
}
