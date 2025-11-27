// Package lawbench measures mathematical properties of performance.
//
// Unlike traditional benchmarks that measure "fast vs slow", lawbench measures
// scalability properties using the Universal Scalability Law (USL):
//
//	C(N) = λN / (1 + α(N-1) + βN(N-1))
//
// Where:
//   - λ (lambda): Serial performance (throughput at N=1)
//   - α (alpha): Contention coefficient (lock waiting)
//   - β (beta): Coordination coefficient (cache coherency, communication)
//   - N: Number of concurrent workers
//
// CRITICAL: Contention measurement depends on GOMAXPROCS.
// If N > GOMAXPROCS, you measure Go scheduler context switching overhead.
// If N ≤ GOMAXPROCS, you measure true application lock contention.
// Set GOMAXPROCS = runtime.NumCPU() for realistic measurement.
//
// Properties measured:
//   - Zero Contention: α < 0.01 (lock-free)
//   - Zero Coordination: β < 0.01 (no communication overhead)
//   - Linear Scaling: C(N) ≈ λN (ideal parallelism)
//   - No Retrograde: C'(N) > 0 (throughput always increases)
//
// Future extensions:
//   - Feigenbaum bifurcation analysis (chaos theory for stability boundaries)
//   - Period-doubling detection (stable → periodic → chaotic transitions)
//   - Lyapunov exponent measurement (quantify chaos)
package lawbench

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Operation represents a benchmarked operation.
// Implementations should be stateless and safe for concurrent execution.
type Operation func(ctx context.Context) error

// Result contains measurements from a single concurrency level.
type Result struct {
	N          int             // Number of concurrent workers
	Duration   time.Duration   // Total benchmark duration
	Operations int64           // Total operations completed
	Throughput float64         // Operations per second
	Latencies  []time.Duration // Individual operation latencies (for percentiles)
	Errors     int64           // Number of failed operations
}

// Statistics contains percentile latency data.
type Statistics struct {
	Mean   time.Duration
	Stddev time.Duration
	P50    time.Duration
	P95    time.Duration
	P99    time.Duration
}

// USLCoefficients contains the Universal Scalability Law parameters.
type USLCoefficients struct {
	Lambda   float64 // λ: Serial throughput (ops/sec at N=1)
	Alpha    float64 // α: Contention coefficient
	Beta     float64 // β: Coordination coefficient
	RSquared float64 // R²: Goodness of fit (1.0 = perfect)
}

// Config controls benchmark execution.
type Config struct {
	Duration time.Duration // How long to run at each concurrency level
	Warmup   time.Duration // Warmup period before measurement
	Levels   []int         // Concurrency levels to test (default: [1,2,4,8,16])
	MaxProcs int           // GOMAXPROCS limit (0 = use runtime default)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Duration: 5 * time.Second,
		Warmup:   1 * time.Second,
		Levels:   []int{1, 2, 4, 8, 16},
		MaxProcs: 0,
	}
}

// Run executes the operation at multiple concurrency levels and returns results.
func Run(ctx context.Context, op Operation, cfg Config) ([]Result, error) {
	if cfg.MaxProcs > 0 {
		oldMaxProcs := runtime.GOMAXPROCS(cfg.MaxProcs)
		defer runtime.GOMAXPROCS(oldMaxProcs)
	}

	results := make([]Result, 0, len(cfg.Levels))

	for _, n := range cfg.Levels {
		result, err := runAtLevel(ctx, op, n, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed at N=%d: %w", n, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// runAtLevel executes the operation with N concurrent workers.
func runAtLevel(ctx context.Context, op Operation, n int, cfg Config) (Result, error) {
	// Warmup phase
	if cfg.Warmup > 0 {
		warmupCtx, cancel := context.WithTimeout(ctx, cfg.Warmup)
		_ = runPhase(warmupCtx, op, n, cfg.Warmup)
		cancel()
	}

	// Measurement phase
	measureCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	return runPhase(measureCtx, op, n, cfg.Duration), nil
}

// runPhase executes the actual benchmark measurement.
func runPhase(ctx context.Context, op Operation, n int, duration time.Duration) Result {
	var (
		wg         sync.WaitGroup
		operations int64
		errors     int64
		latencies  = make([][]time.Duration, n) // Per-worker latency slices
	)

	start := time.Now()

	for i := 0; i < n; i++ {
		wg.Add(1)
		workerID := i
		latencies[workerID] = make([]time.Duration, 0, 1000)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					opStart := time.Now()
					err := op(ctx)
					opDuration := time.Since(opStart)

					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&operations, 1)
						latencies[workerID] = append(latencies[workerID], opDuration)
					}
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Merge latencies from all workers
	allLatencies := make([]time.Duration, 0, operations)
	for _, workerLatencies := range latencies {
		allLatencies = append(allLatencies, workerLatencies...)
	}

	throughput := float64(operations) / elapsed.Seconds()

	return Result{
		N:          n,
		Duration:   elapsed,
		Operations: operations,
		Throughput: throughput,
		Latencies:  allLatencies,
		Errors:     errors,
	}
}

// CalculateStatistics computes percentile latencies.
func CalculateStatistics(result Result) Statistics {
	if len(result.Latencies) == 0 {
		return Statistics{}
	}

	sorted := make([]time.Duration, len(result.Latencies))
	copy(sorted, result.Latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Mean
	var sum time.Duration
	for _, lat := range sorted {
		sum += lat
	}
	mean := sum / time.Duration(len(sorted))

	// Standard deviation
	var variance float64
	for _, lat := range sorted {
		diff := float64(lat - mean)
		variance += diff * diff
	}
	stddev := time.Duration(math.Sqrt(variance / float64(len(sorted))))

	// Percentiles
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	return Statistics{
		Mean:   mean,
		Stddev: stddev,
		P50:    p50,
		P95:    p95,
		P99:    p99,
	}
}

// FitUSL performs nonlinear regression to find λ, α, β coefficients.
//
// Uses linearization approach: transform USL to linear form and solve analytically.
// For C(N) = λN / (1 + α(N-1) + βN(N-1)), rearrange to:
//
//	N/C(N) = 1/λ + (α/λ)(N-1) + (β/λ)N(N-1)
//
// This is linear in 1/λ, α/λ, β/λ. Solve via least squares, then recover λ, α, β.
//
// Returns coefficients and R² goodness of fit.
func FitUSL(results []Result) (USLCoefficients, error) {
	if len(results) < 3 {
		return USLCoefficients{}, fmt.Errorf("need at least 3 data points, got %d", len(results))
	}

	// Build design matrix and response vector for linear system
	// Y = N/C(N), X = [1, (N-1), N(N-1)]
	// Solve: Y = b0 + b1*(N-1) + b2*N*(N-1)
	//
	// Then: λ = 1/b0, α = b1/b0, β = b2/b0

	var sumY, sumX1, sumX2, sumX1X1, sumX2X2, sumX1X2, sumYX1, sumYX2 float64
	var sumOne float64

	for _, r := range results {
		if r.Throughput == 0 {
			continue
		}

		N := float64(r.N)
		Y := N / r.Throughput // N/C(N)
		X1 := N - 1           // (N-1)
		X2 := N * (N - 1)     // N(N-1)

		sumY += Y
		sumX1 += X1
		sumX2 += X2
		sumX1X1 += X1 * X1
		sumX2X2 += X2 * X2
		sumX1X2 += X1 * X2
		sumYX1 += Y * X1
		sumYX2 += Y * X2
		sumOne += 1
	}

	// Solve 3x3 system using Cramer's rule
	// [n    sumX1    sumX2  ] [b0]   [sumY  ]
	// [sumX1 sumX1X1 sumX1X2] [b1] = [sumYX1]
	// [sumX2 sumX1X2 sumX2X2] [b2]   [sumYX2]

	det := sumOne*(sumX1X1*sumX2X2-sumX1X2*sumX1X2) -
		sumX1*(sumX1*sumX2X2-sumX1X2*sumX2) +
		sumX2*(sumX1*sumX1X2-sumX1X1*sumX2)

	if math.Abs(det) < 1e-10 {
		// Fallback: use simple heuristic estimation
		lambda := results[0].Throughput
		return USLCoefficients{
			Lambda:   lambda,
			Alpha:    0.01,
			Beta:     0.0,
			RSquared: 0.0,
		}, nil
	}

	// Calculate b0, b1, b2 using Cramer's rule
	det0 := sumY*(sumX1X1*sumX2X2-sumX1X2*sumX1X2) -
		sumX1*(sumYX1*sumX2X2-sumX1X2*sumYX2) +
		sumX2*(sumYX1*sumX1X2-sumX1X1*sumYX2)

	det1 := sumOne*(sumYX1*sumX2X2-sumX1X2*sumYX2) -
		sumY*(sumX1*sumX2X2-sumX1X2*sumX2) +
		sumX2*(sumX1*sumYX2-sumYX1*sumX2)

	det2 := sumOne*(sumX1X1*sumYX2-sumYX1*sumX1X2) -
		sumX1*(sumX1*sumYX2-sumYX1*sumX2) +
		sumY*(sumX1*sumX1X2-sumX1X1*sumX2)

	b0 := det0 / det
	b1 := det1 / det
	b2 := det2 / det

	// Recover λ, α, β from linear coefficients
	lambda := 1.0 / b0
	alpha := b1 / b0
	beta := b2 / b0

	// CRITICAL FIX: Detect negative beta (linearization artifact)
	// β < 0 is mathematically impossible in USL unless superlinear scaling
	// (cache friendliness, rare). Usually indicates fitting error from noise.
	// Fallback to 2-parameter model (λ, α only) when β < 0.
	if beta < 0 && alpha > 0 {
		// Re-fit with β = 0 (contention-only model)
		// Y = b0 + b1*(N-1), solve 2x2 system
		var sum2Y, sum2X1, sum2X1X1, sum2YX1, sum2One float64
		for _, r := range results {
			if r.Throughput == 0 {
				continue
			}
			N := float64(r.N)
			Y := N / r.Throughput
			X1 := N - 1
			sum2Y += Y
			sum2X1 += X1
			sum2X1X1 += X1 * X1
			sum2YX1 += Y * X1
			sum2One += 1
		}

		det2 := sum2One*sum2X1X1 - sum2X1*sum2X1
		if math.Abs(det2) > 1e-10 {
			b0_new := (sum2X1X1*sum2Y - sum2X1*sum2YX1) / det2
			b1_new := (sum2One*sum2YX1 - sum2X1*sum2Y) / det2
			lambda = 1.0 / b0_new
			alpha = b1_new / b0_new
			beta = 0.0 // Clamped
		}
	}

	// Calculate R² (coefficient of determination)
	var ssRes, ssTot float64
	var meanThroughput float64
	for _, r := range results {
		meanThroughput += r.Throughput
	}
	meanThroughput /= float64(len(results))

	for _, r := range results {
		predicted := uslModel(float64(r.N), lambda, alpha, beta)
		ssRes += (r.Throughput - predicted) * (r.Throughput - predicted)
		ssTot += (r.Throughput - meanThroughput) * (r.Throughput - meanThroughput)
	}

	rSquared := 1 - (ssRes / ssTot)

	return USLCoefficients{
		Lambda:   lambda,
		Alpha:    alpha,
		Beta:     beta,
		RSquared: rSquared,
	}, nil
}

// uslModel calculates predicted throughput using USL formula.
func uslModel(n, lambda, alpha, beta float64) float64 {
	return (lambda * n) / (1 + alpha*(n-1) + beta*n*(n-1))
}

// PredictThroughput estimates throughput at a given concurrency level.
func (c USLCoefficients) PredictThroughput(n int) float64 {
	return uslModel(float64(n), c.Lambda, c.Alpha, c.Beta)
}

// Efficiency returns the ratio of actual to ideal throughput.
// 1.0 = perfect linear scaling, <1.0 = contention/coordination overhead.
func (c USLCoefficients) Efficiency(n int) float64 {
	predicted := c.PredictThroughput(n)
	ideal := c.Lambda * float64(n)
	if ideal == 0 {
		return 0
	}
	return predicted / ideal
}
