// Package lawbench provides adaptive load management for distributed systems.
//
// # Overview
//
// lawbench prevents cascade failures through intelligent load shedding based on real-time
// system coupling metrics. It monitors the r-parameter (system coupling coefficient) and
// applies load shedding before instability occurs.
//
// # Architecture
//
// The package components:
//
//   - benchmark/    - Universal Scalability Law (USL) measurement
//   - governor/     - Load shedding controller
//   - criticality/  - Capacity limit detection
//   - feigenbaum/   - Instability threshold analysis
//   - runtime/      - Runtime law verification
//   - assertions/   - Test helpers for scalability properties
//
// # Quick Start
//
// Measure scalability properties of an operation:
//
//	op := func(ctx context.Context) error {
//	    // Your operation here
//	    return doWork()
//	}
//
//	results, err := lawbench.Run(ctx, op, lawbench.DefaultConfig())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Fit to Universal Scalability Law
//	usl, err := lawbench.FitUSL(results)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Contention (α): %.4f\n", usl.Alpha)
//	fmt.Printf("Coherency (β): %.4f\n", usl.Beta)
//
// # The Governor
//
// The Governor monitors system coupling (r) and sheds load when approaching instability:
//
//	governor := lawbench.NewGovernor(1.5) // Initial r = 1.5
//
//	// Update with current system state
//	action := governor.Update(currentR, alpha, beta, concurrency)
//
//	switch action.Type {
//	case lawbench.ActionStable:
//	    // System healthy, no action needed
//	case lawbench.ActionWarning:
//	    log.Printf("WARNING: r = %.2f, approaching instability", action.CurrentR)
//	case lawbench.ActionPacing:
//	    // Shed 10-20% load
//	    shedLoad(0.2)
//	case lawbench.ActionThrottle:
//	    // Emergency: shed 50% load immediately
//	    shedLoad(0.5)
//	case lawbench.ActionBlockDeploy:
//	    // Deployment exceeds capacity limits
//	    return errors.New("deployment blocked: exceeds capacity")
//	}
//
// # The 21% Rule
//
// Capacity constraint based on system stability threshold:
//
//	ΔComplexity / ΔCore ≤ 4.669
//
// Practical meaning: For every unit of core work, you can add ~4.7 units
// of feature work before hitting capacity limits. This prevents r from climbing
// toward instability (r ≥ 3.0).
//
// Example:
//
//	metrics := lawbench.SystemIntegrityMetrics{
//	    DeltaCriticalCore: 100,  // Changed 100 LOC in core
//	    DeltaComplexity:   500,  // Added 500 LOC features
//	}
//
//	ratio := lawbench.CriticalityScalingRatio(metrics)
//	if ratio > 4.669 {
//	    // VIOLATION: Exceeding capacity, r climbing
//	    return errors.New("deployment rejected: exceeds capacity limit")
//	}
//
// # USL: Universal Scalability Law
//
// Dr. Neil Gunther's USL models throughput as a function of concurrency:
//
//	C(N) = λN / (1 + α(N-1) + βN(N-1))
//
// Where:
//   - λ (lambda): Serial performance (throughput at N=1)
//   - α (alpha): Contention coefficient (lock waiting)
//   - β (beta): Coordination coefficient (cache coherency, communication)
//   - N: Number of concurrent workers
//
// Properties measured:
//   - Zero Contention: α < 0.01 (lock-free)
//   - Zero Coordination: β < 0.01 (no communication overhead)
//   - Linear Scaling: C(N) ≈ λN (ideal parallelism)
//   - No Retrograde: C'(N) > 0 (throughput always increases)
//
// # The r-parameter
//
// The r-parameter measures system coupling (how much requests interfere with each other):
//
//	x_{n+1} = r · x_n · (1 - x_n)
//
// System behavior by r value:
//   - r < 1.0:       Underutilized (wasted capacity)
//   - 1.0 < r < 2.0: Stable (predictable)
//   - 2.0 < r < 3.0: Stable (some oscillation)
//   - r ≥ 3.0:       Unstable (unpredictable latency spikes)
//
// Calculate r from USL coefficients:
//
//	r = 1 + 2·α + 5·β·N
//
// This formula connects scalability measurement (USL) to stability prediction.
//
// # Testing
//
// Use assertions to validate scalability properties:
//
//	func TestMyOperation(t *testing.T) {
//	    results := runBenchmark(...)
//
//	    // Assert lock-free (α < 0.01)
//	    lawbench.AssertZeroContention(t, results, lawbench.DefaultAssertionConfig())
//
//	    // Assert linear scaling
//	    lawbench.AssertLinearScaling(t, results, lawbench.DefaultAssertionConfig())
//
//	    // Assert no retrograde (throughput always increases)
//	    lawbench.AssertNoRetrograde(t, results, lawbench.DefaultAssertionConfig())
//	}
//
// # Philosophy
//
// Traditional benchmarks answer: "How fast is this?"
// lawbench answers: "What are the scalability properties?"
//
// - Does it scale linearly? (C(N) ≈ λN)
// - Is it lock-free? (α < 0.01)
// - Does it have coordination overhead? (β > 0)
// - Will it degrade at high concurrency? (C'(N) < 0)
// - What is its distance from instability? (r < 3.0)
//
// This shifts focus from "optimization" (make it faster) to "properties"
// (ensure it scales predictably).
//
// # Production Usage
//
// For HTTP middleware integration:
//
//	governor := lawbench.NewGovernor(1.5)
//
//	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
//	    start := time.Now()
//
//	    // Your handler logic
//	    handleRequest(w, r)
//
//	    // Update governor with latency
//	    latency := time.Since(start)
//	    currentR := calculateR(latency, errorRate)
//	    action := governor.Update(currentR, alpha, beta, activeRequests)
//
//	    if action.Type == lawbench.ActionThrottle {
//	        // Shed load: reject new requests with 503
//	        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
//	    }
//	})
//
// # See Also
//
//   - docs/LAWBENCH.md - USL testing guide
//   - docs/FEIGENBAUM.md - Instability threshold analysis
//   - docs/CRITICALITY_SCALING.md - The 21% Rule explained
//   - docs/RUNTIME_LAW_TESTING.md - Runtime verification
//   - examples/ - Working code samples
package lawbench
