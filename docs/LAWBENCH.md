# lawbench: Mathematical Performance Testing

**lawbench** measures scalability properties using mathematical models, not just "fast vs slow" comparisons. It's designed to complement [lawtest](https://github.com/alexshd/lawtest) (property-based testing) with scalability laws.

## Philosophy

Traditional benchmarks answer: "How fast is this?"  
**lawbench** answers: "What mathematical properties does the performance exhibit?"

- Does it scale linearly? (C(N) ≈ λN)
- Is it lock-free? (α < 0.01)
- Does it have cache coherency overhead? (β > 0)
- Will it retrograde at high concurrency? (C'(N) < 0)

## Universal Scalability Law (USL)

lawbench uses Dr. Neil Gunther's **Universal Scalability Law**:

```
C(N) = λN / (1 + α(N-1) + βN(N-1))
```

Where:

- **N**: Number of concurrent workers
- **C(N)**: Throughput at concurrency N (ops/sec)
- **λ (lambda)**: Serial performance (throughput at N=1)
- **α (alpha)**: Contention coefficient (lock waiting)
- **β (beta)**: Coordination coefficient (cache coherency, communication)

### Interpreting Coefficients

**α (Contention)**:

- α < 0.01: Excellent (lock-free or efficient locks)
- α < 0.05: Good (minimal lock contention)
- α ≥ 0.05: Poor (significant lock bottleneck)

**β (Coordination)**:

- β < 0: Superlinear scaling (cache-friendly, batching benefits)
- β < 0.01: Excellent (minimal cache coherency traffic)
- β < 0.05: Good (some communication overhead)
- β ≥ 0.05: Poor (severe cache/communication bottleneck)

**R² (Goodness of Fit)**:

- R² > 0.98: Excellent (USL model fits perfectly)
- R² > 0.95: Good (model explains the data well)
- R² > 0.90: Fair (some measurement noise)
- R² < 0.90: Poor (check for measurement artifacts)

## Usage

### Basic Measurement

```go
import (
    "context"
    "testing"
    "github.com/alexshd/trdynamics/lawbench"
)

func TestMyOperation_Scalability(t *testing.T) {
    // Define operation to measure
    op := func(ctx context.Context) error {
        // Your code here
        return mySerializationFunction()
    }

    // Configure benchmark
    cfg := lawbench.DefaultConfig()
    cfg.Duration = 5 * time.Second
    cfg.Levels = []int{1, 2, 4, 8, 16}

    // Run measurement
    results, err := lawbench.Run(context.Background(), op, cfg)
    if err != nil {
        t.Fatalf("Benchmark failed: %v", err)
    }

    // Assert scalability properties
    lawbench.AssertScalability(t, results)
}
```

### Custom Assertions

```go
func TestMyOperation_ZeroContention(t *testing.T) {
    op := func(ctx context.Context) error {
        return lockFreeOperation()
    }

    cfg := lawbench.DefaultConfig()
    results, _ := lawbench.Run(context.Background(), op, cfg)

    // Assert specific property
    assertCfg := lawbench.DefaultAssertionConfig()
    assertCfg.MaxContention = 0.01  // Require α < 0.01

    lawbench.AssertZeroContention(t, results, assertCfg)
}
```

### Capacity Planning

```go
func TestMyOperation_CapacityPlanning(t *testing.T) {
    op := func(ctx context.Context) error {
        return myOperation()
    }

    cfg := lawbench.DefaultConfig()
    results, _ := lawbench.Run(context.Background(), op, cfg)

    coeffs, _ := lawbench.FitUSL(results)

    // Predict throughput at higher concurrency
    predicted32 := coeffs.PredictThroughput(32)
    predicted64 := coeffs.PredictThroughput(64)

    t.Logf("N=32: %.2f ops/sec (efficiency: %.1f%%)",
        predicted32, coeffs.Efficiency(32)*100)
    t.Logf("N=64: %.2f ops/sec (efficiency: %.1f%%)",
        predicted64, coeffs.Efficiency(64)*100)
}
```

## Real-World Example: Cap'n Proto

From `hive/wire/event_capnp_lawbench_test.go`:

### Event Deserialization

```
λ = 845,640 ops/sec (serial performance)
α = 0.323 (moderate contention from allocator)
β = 0.013 (minimal coordination overhead)
R² = 0.975 (excellent model fit)

Efficiency:
  N=1:  100% (845K ops/sec)
  N=2:   74% (1.25M ops/sec)
  N=4:   47% (1.59M ops/sec)
  N=8:   25% (1.69M ops/sec)
```

**Interpretation**: Deserialization scales well to N=8 but shows allocator contention (α = 0.32). Coordination is minimal (β = 0.01). At N=64, efficiency drops to 1.3% - don't scale beyond N=8.

### Packet Batch100

```
λ = 18,741 ops/sec (serial performance)
α = -0.005 (ZERO contention - lock-free!)
β = 0.154 (high coordination from large allocations)
R² = 0.922 (good model fit)

Efficiency:
  N=1:  100% (18.7K ops/sec)
  N=2:   77% (28.8K ops/sec)
  N=4:   35% (26.5K ops/sec)
  N=8:   10% (15.6K ops/sec)  ← Retrograde!
```

**Interpretation**: Large batches are lock-free (α ≈ 0) but suffer from coordination overhead (β = 0.15). System becomes retrograde at N>4. Use batching for throughput, but keep N ≤ 4.

## API Reference

### Core Types

```go
type Operation func(ctx context.Context) error

type Result struct {
    N          int           // Concurrency level
    Duration   time.Duration // Measurement duration
    Operations int64         // Total operations
    Throughput float64       // Ops/sec
    Latencies  []time.Duration // For percentiles
}

type USLCoefficients struct {
    Lambda   float64  // λ: Serial performance
    Alpha    float64  // α: Contention
    Beta     float64  // β: Coordination
    RSquared float64  // R²: Goodness of fit
}
```

### Functions

```go
// Run executes operation at multiple concurrency levels
func Run(ctx context.Context, op Operation, cfg Config) ([]Result, error)

// FitUSL performs nonlinear regression to find λ, α, β
func FitUSL(results []Result) (USLCoefficients, error)

// Predict throughput at given concurrency
func (c USLCoefficients) PredictThroughput(n int) float64

// Calculate efficiency (actual / ideal throughput)
func (c USLCoefficients) Efficiency(n int) float64
```

### Assertions

```go
// Assert α < threshold (lock-free property)
func AssertZeroContention(t *testing.T, results []Result, cfg AssertionConfig)

// Assert β < threshold (no coordination overhead)
func AssertZeroCoordination(t *testing.T, results []Result, cfg AssertionConfig)

// Assert efficiency > threshold at all N (linear scaling)
func AssertLinearScaling(t *testing.T, results []Result, cfg AssertionConfig)

// Assert C(N+1) > C(N) for all N (monotonic throughput)
func AssertNoRetrograde(t *testing.T, results []Result, cfg AssertionConfig)

// Run all assertions (comprehensive check)
func AssertScalability(t *testing.T, results []Result)

// Print detailed USL analysis
func PrintAnalysis(t *testing.T, results []Result)
```

## Mathematical Properties

lawbench measures these **algebraic properties** of performance:

### 1. Zero Contention (α ≈ 0)

**Property**: ∂C/∂N ≈ λ when α ≈ 0  
**Meaning**: Throughput grows linearly with workers. No lock waiting.  
**Test**: `AssertZeroContention(t, results, cfg)`

### 2. Zero Coordination (β ≈ 0)

**Property**: C(N) ≈ λN when β ≈ 0  
**Meaning**: No quadratic slowdown. No cache coherency traffic.  
**Test**: `AssertZeroCoordination(t, results, cfg)`

### 3. Linear Scaling (Efficiency ≈ 1)

**Property**: C(N) / (λN) > 0.95 for all N  
**Meaning**: Actual throughput ≈ ideal throughput.  
**Test**: `AssertLinearScaling(t, results, cfg)`

### 4. No Retrograde (C'(N) > 0)

**Property**: ∂C/∂N > 0 for all N  
**Meaning**: Throughput never decreases with more workers.  
**Test**: `AssertNoRetrograde(t, results, cfg)`

## Future: Feigenbaum Bifurcation Analysis

**Phase 2** (roadmap): Measure **chaos boundaries** using Feigenbaum bifurcation theory.

### Concept

As load increases 0% → 100%, distributed systems exhibit:

1. **Stable** (<75% load): Latency constant (3μs, 3μs, 3μs)
2. **Period-2** (75-90%): Latency alternates (3μs, 5μs, 3μs, 5μs)
3. **Period-4** (90-95%): Complex patterns (3μs, 4μs, 6μs, 8μs)
4. **Period-8** (95-98%): More complex
5. **Chaos** (>98%): Unpredictable (3μs, 50μs, 2μs, 100μs)

### Feigenbaum Constants (Universal)

These constants appear in **all** systems undergoing period-doubling:

- **δ (delta) ≈ 4.669**: Rate of period-doubling  
  `LoadChaos - Load₄ = (Load₄ - Load₂) / δ`

- **α (alpha) ≈ 2.502**: Amplitude scaling  
  `Amplitude₄ / Amplitude₂ ≈ α`

### Lyapunov Exponent (λ)

Measures rate of chaos:

- λ < 0: Stable (perturbations decay)
- λ = 0: Periodic (neutral)
- λ > 0: Chaotic (perturbations grow exponentially)

### Proposed API (Future)

```go
// Find load where system becomes chaotic
func FindChaosBoundary(t *testing.T, op Operation) float64

// Map all bifurcation points
func FindBifurcationPoints(t *testing.T, op Operation) []float64

// Validate universal constants
func AssertFeigenbaumConstants(t *testing.T, analysis BifurcationAnalysis)

// Calculate Lyapunov exponent at load
func CalculateLyapunovExponent(t *testing.T, op Operation, load float64) float64

// Automatic headroom calculation
// Returns: If chaos at 97.3%, operate at 97.3%/3 = 32.4% max
func CalculateSafeOperatingPoint(t *testing.T, op Operation) float64
```

### Applications

1. **Early Warning System**: Monitor λ in production to detect approaching chaos
2. **Automatic Headroom**: Calculate safe operating region (1/3 of chaos boundary)
3. **Load Testing Guidance**: Test at 50% (stable), 65% (production), 78% (first bifurcation)
4. **Strange Attractors**: Visualize fractal latency patterns in chaos regime

### Why This Matters

Feigenbaum's discovery: **Same δ, α apply to**:

- Logistic map (mathematics)
- Weather systems (meteorology)
- Population dynamics (biology)
- **Distributed systems** (software!)

This is a **universal law of nature**. lawbench would be the first tool to apply it to performance engineering.

## Design Philosophy

lawbench follows the same philosophy as lawtest:

1. **Properties over Examples**: Test mathematical properties, not specific values
2. **Algebraic Laws**: Commutativity, associativity, idempotence → contention, coordination, efficiency
3. **Black Box**: Works on any system (if designed with lawtest principles)
4. **Production Insights**: Predict behavior at scale from small measurements

## Why "lawbench"?

- **law**: Mathematical laws (USL, Feigenbaum)
- **bench**: Benchmarking performance
- **Complements lawtest**: Properties for correctness + properties for performance

## References

- **Universal Scalability Law**: Gunther, N. J. (2007). _Guerrilla Capacity Planning_
- **USL Model**: C(N) = λN / (1 + α(N-1) + βN(N-1))
- **Feigenbaum Constants**: Feigenbaum, M. J. (1978). "Quantitative universality for a class of nonlinear transformations"
- **lawtest**: https://github.com/alexshd/lawtest

## Contributing

lawbench is part of the trdynamics project. Future contributions:

1. **Feigenbaum Analysis** (Phase 2): Implement bifurcation detection
2. **Load Injection**: Framework for controlled load ramps 0% → 110%
3. **Visualization**: Bifurcation diagrams, strange attractors
4. **Production Monitoring**: Real-time λ calculation for early warning

---

**Status**: ✅ Phase 1 Complete (USL measurement and assertions)  
**Next**: 🚧 Phase 2 - Feigenbaum bifurcation analysis (roadmap)
