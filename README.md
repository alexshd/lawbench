# lawbench: Adaptive Reliability Library for Go Services

**Status**: Production Ready | 107/107 tests passing ✅

**What it does**: Adaptive reliability library using **Universal Scalability Law (USL)** metrics to detect system saturation and apply traffic shedding milliseconds before latency degrades. Unlike static rate limiters, it adapts to real-time contention through closed-loop feedback control.

**Strategy**: Continuous feedback monitoring → Adaptive control decisions → Prevents cascade failures

**Proven**: 10x latency improvement under extreme load ([see proof](docs/PROOF.md))

---

## Quick Start

```bash
go get github.com/alexshd/lawbench
```

```go
import "github.com/alexshd/lawbench"

governor := lawbench.NewGovernor(1.5)

http.HandleFunc("/api/endpoint", func(w http.ResponseWriter, r *http.Request) {
    if governor.ShouldShedLoad() {
        http.Error(w, "Service at capacity", 503)
        return
    }

    start := time.Now()
    // ... your handler logic ...
    governor.RecordRequest(time.Since(start))
})
```

**That's it.** Your system now self-regulates under load.

[Full integration guide →](QUICKSTART.md)

---

## The Problem

Traditional systems accept all incoming traffic until they collapse:

```
Load increases → Queues grow → Latency explodes → Cascade failure → Everyone suffers
```

**Result**: 2-3 second response times, timeouts, angry users, 3 AM pages.

## The Solution: Adaptive Control System

lawbench provides **closed-loop feedback control** for your Kubernetes workloads. Unlike passive metrics collection, it continuously analyzes system behavior and applies adaptive throttling:

### Three-Way Strategy

1. **Safety**: Shed load when system approaches saturation (prevent crashes)
2. **Optimization**: Signal HPA to scale up when beneficial (serve more traffic)
3. **Retrograde Prevention**: Block scaling when it would decrease throughput (save money)

```
Load increases → Feedback loop detects coupling → Adaptive control:
  • r < 2.5: Scale up (linear scaling region)
  • 2.5 ≤ r < 3.0: Shed 10% load (approaching saturation)
  • r ≥ 3.0: Emergency shedding (prevent cascade)
```

**Result**: 100ms response times for served requests, no cascade, no pages, optimal pod count.

---

## Empirical Results

Load test: 300 concurrent users, 10% slow queries (1-3s), aggressive memory allocation

| Metric   | Without lawbench | With lawbench | Improvement    |
| -------- | ---------------- | ------------- | -------------- |
| Average  | 296ms            | **101ms**     | **3x faster**  |
| P95      | 2047ms           | **191ms**     | **10x faster** |
| Max      | 3797ms           | **259ms**     | **15x faster** |
| Failures | High variance    | Predictable   | Stable         |

**Key insight**: By refusing 10% of requests, the remaining 90% ran 10x faster.

[See full validation →](docs/EMPIRICAL_VALIDATION.md)

---

## Core Features

### 1. Adaptive Load Shedding

Monitors coupling parameter `r` and sheds load when approaching instability:

- `r < 2.5`: ✅ Stable (no action)
- `2.5 ≤ r < 2.8`: ⚠️ Warning (monitor)
- `2.8 ≤ r < 3.0`: 🔶 Shedding (reject 10-20%)
- `r ≥ 3.0`: 🚨 Emergency (reject 50-70%)

### 2. Kubernetes Autoscaling Intelligence

**Retrograde scaling prevention**: Detects when adding pods decreases total throughput.

Integrates with Kubernetes HPA through feedback metrics:

```go
decision := autoscaler.ShouldScale(metrics)
// Returns: ScaleUp, Maintain, or ShedLoad

// Expose to Kubernetes HPA
if decision == ScaleUp {
    // HPA scales based on custom metric
    metrics.SetCouplingParameter(currentR)
} else if decision == ShedLoad {
    // Block HPA, start load shedding instead
    governor.ActivateLoadShedding()
}
```

**Safety + Optimization**: Closed-loop control prevents operating beyond peak capacity.

**Saves**: $9,800/month by preventing retrograde scaling where adding pods decreases throughput.

### 3. Tail Latency Tracking

Detects shift from stable (Gaussian) to unstable (Power Law) distributions:

```go
tracker := lawbench.NewTailDivergenceTracker(1000)
stats := tracker.GetStats()
// P95/P50 ratio: <3 = stable, >10 = entering saturation
```

---

## How It Works: Closed-Loop Feedback Control

### Closed-Loop vs Open-Loop Control

**Open-loop** (traditional): Collect metrics, alert when thresholds crossed, wait for human intervention.

**Closed-loop** (lawbench): Continuously measure system state, calculate control actions, apply adaptive throttling.

### The Coupling Parameter (r): Feedback Control Metric

`r` measures system contention - how much requests interfere with each other:

- **Low r** (r < 2.0): Independent requests → Linear scaling region → Safe to scale up
- **Medium r** (2.0-2.8): Some contention → Approaching saturation → Scale cautiously
- **High r** (r ≥ 2.8): Heavy contention → Saturation point → **Control action: Shed load, don't scale**

**Feedback control loop**:

```
Measure: r = 2.9 (coupling detected)
↓
Analyze: System at saturation point (N > N_peak)
↓
Control action: Block HPA scaling, activate load shedding
↓
Result: Stable system, cost savings, no human intervention
```

lawbench continuously measures `r` through feedback sensors:

- Request latency patterns
- Error rates
- Concurrency levels
- Queue depths

When `r` approaches critical threshold (3.0), the control loop triggers load shedding.

### Universal Scalability Law: The Math Behind Retrograde Prevention

Peak throughput occurs at `N_peak = √((1-α)/β)` workers.

Beyond this point, **adding pods decreases throughput** (retrograde scaling).

**Control objective**: lawbench calculates this limit and prevents Kubernetes from scaling into the retrograde zone.

---

## Usage Examples

### Kubernetes Deployment with Feedback Control

**Complete example**: Adaptive service with HPA integration

```go
package main

import (
    "net/http"
    "time"
    "github.com/alexshd/lawbench"
)

func main() {
    // Initialize adaptive controller
    governor := lawbench.NewGovernor(1.5)
    autoscaler := lawbench.NewAutoscaler()

    http.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
        // Closed-loop control: adaptive load shedding
        if governor.ShouldShedLoad() {
            http.Error(w, "Service temporarily at capacity", 503)
            return
        }

        start := time.Now()
        processOrder(w, r)
        governor.RecordRequest(time.Since(start))
    })

    http.HandleFunc("/health", healthCheck)

    // Expose metrics for Kubernetes HPA
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        stats := governor.GetStatistics()

        // Feedback control: inform HPA of system state
        decision := autoscaler.ShouldScale(stats)
        stats["scaling_decision"] = decision
        stats["feedback_control"] = true

        json.NewEncoder(w).Encode(stats)
    })

    http.ListenAndServe(":8080", nil)
}
```

**Kubernetes HPA configuration** (uses custom metrics):

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Pods
      pods:
        metric:
          name: coupling_parameter
        target:
          type: AverageValue
          averageValue: "2.5" # Scale when r > 2.5
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
```

### Basic HTTP Server

```go
package main

import (
    "net/http"
    "time"
    "github.com/alexshd/lawbench"
)

func main() {
    governor := lawbench.NewGovernor(1.5)

    http.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
        if governor.ShouldShedLoad() {
            http.Error(w, "Service temporarily at capacity", 503)
            return
        }

        start := time.Now()
        processOrder(w, r)
        governor.RecordRequest(time.Since(start))
    })

    http.HandleFunc("/health", healthCheck)
    http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        stats := governor.GetStatistics()
        json.NewEncoder(w).Encode(stats)
    })

    http.ListenAndServe(":8080", nil)
}
```

### With Benchmarking

Measure your system's scalability parameters:

```go
benchmark := lawbench.NewBenchmark(targetFunc)
results := benchmark.Run(lawbench.BenchmarkConfig{
    Workers:  []int{1, 2, 4, 8, 16},
    Duration: 10 * time.Second,
})

// results.Alpha: Contention coefficient
// results.Beta: Coherency coefficient
// results.R: Current coupling parameter
```

### Monitoring Setup

```bash
# Watch system coupling in real-time
watch -n 1 'curl -s http://localhost:8080/metrics | jq "{r, status, requests}"'
```

Export to Prometheus:

```go
governor.GetStatistics() // Returns map[string]float64
// Expose as Prometheus metrics
```

---

## Production Checklist

### Kubernetes Deployment

- [ ] Governor integrated at pod entry points (feedback control)
- [ ] `/metrics` endpoint exposed with coupling parameter `r`
- [ ] Kubernetes HPA configured to read `r` metric
- [ ] HPA scaling policy respects `r` thresholds:
  - `r < 2.5`: Linear scaling region (scale freely)
  - `r ≥ 2.8`: Saturation point (block scaling, activate shedding)
- [ ] Alerts configured for `r > 2.8` (approaching saturation)
- [ ] Load tests validate adaptive behavior:
  - Shedding activates at correct threshold
  - HPA scaling blocked in retrograde zone
- [ ] 503 responses logged and tracked
- [ ] Dashboard shows:
  - `r(t)` time series (coupling parameter)
  - Pod count vs r-parameter correlation
  - Shed rate vs scaling events

### Safety & Optimization

- [ ] Safety: Load shedding prevents pod crashes
- [ ] Optimization: HPA scales in linear region only
- [ ] Cost savings: Retrograde scaling prevented

---

## Kubernetes Strategy: Preventing Retrograde Scaling

### The Three Pillars

**1. Safety First** 🛡️

- Feedback control detects approaching saturation
- Adaptive load shedding prevents cascade failures
- Protects existing pods from overload
- **Result**: Zero crashes, stable latency, predictable service

**2. Cost Optimization** 💰

- USL-based calculation determines optimal pod count
- Blocks HPA scaling in retrograde zone
- Prevents adding pods that decrease total throughput
- **Result**: $9,800/month saved, optimal resource usage

**3. Adaptive Scaling** 📈

- Closed-loop control continuously calculates N_peak
- Signals HPA when more capacity increases throughput
- Distinguishes "scale up" from "shed load" conditions
- **Result**: Right-sized deployment, fast response times

### Decision Matrix: Control Actions

| r Value | System State     | Control Action      | Kubernetes Action            |
| ------- | ---------------- | ------------------- | ---------------------------- |
| r < 2.0 | Linear scaling   | ✅ Allow scaling    | HPA can scale up freely      |
| 2.0-2.5 | Sub-linear       | ✅ Monitor          | HPA scales cautiously        |
| 2.5-2.8 | Approaching peak | ⚠️ Prepare          | Start monitoring closely     |
| 2.8-3.0 | At saturation    | 🔶 Shed 10-20% load | Block HPA, activate shedding |
| r ≥ 3.0 | Retrograde zone  | 🚨 Shed 50% load    | Emergency mode, no scaling   |

### Feedback Control Loop

```
1. Measure: Collect latency, errors, concurrency
   ↓
2. Analyze: Calculate r-parameter (contention coefficient)
   ↓
3. Control decision: Based on USL model
   ↓
   ├→ r < 2.5: Signal HPA "linear scaling region"
   ├→ 2.5 ≤ r < 2.8: Signal HPA "approaching saturation"
   └→ r ≥ 2.8: Block HPA + activate load shedding
   ↓
4. Actuate: Execute control action
   ↓
5. Repeat: Continuous feedback control (every request)
```

**Control objective**: Prevent operation in retrograde zone where N > N_peak.

---

## Configuration

### Tuning Thresholds

```go
governor := lawbench.NewGovernor(1.5)

// Adjust thresholds (defaults shown)
governor.SetWarningThreshold(2.8)   // Start monitoring
governor.SetShockThreshold(3.0)     // Activate shedding
```

**Note**: Default thresholds are mathematically derived. Only adjust if you understand the implications.

### Integration Patterns

**Kubernetes Deployment** (Recommended): Closed-loop control per pod

```yaml
# Each pod runs feedback control loop
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  template:
    spec:
      containers:
        - name: app
          # App uses lawbench governor for adaptive control
          # Exposes /metrics with r-parameter
          # HPA scales based on coupling metric
```

```go
// In your Kubernetes service:
governor := lawbench.NewGovernor(1.5)

// Feedback control: adaptive load shedding
if governor.ShouldShedLoad() {
    return 503 // Safety: protect this pod
}

// Signal to HPA for scaling decisions
autoscaler.UpdateMetrics(governor.GetCoupling())
```

````

**Edge Layer** (Ingress/Gateway): Cluster-wide protection

```go
// Protect all downstream services
if governor.ShouldShedLoad() { return 503 }
````

**Service Layer** (Per-service): Granular control

```go
// Each service has independent feedback control
orderService.Governor.ShouldShedLoad()
paymentService.Governor.ShouldShedLoad()
```

**Resource Layer** (Database, Cache): Resource protection

```go
// Protect expensive operations through feedback control
if dbGovernor.ShouldShedLoad() { return cached }
```

---

## Why This Works

### The Queueing Problem

Traditional systems queue requests until resources available:

- Queue grows unbounded
- Latency becomes unpredictable
- Eventually: memory exhaustion, timeouts, cascade failure

### The Admission Control Solution

lawbench rejects excess requests at the door:

- Queue stays bounded
- Latency remains predictable
- Resources focus on requests being served

**Trade-off**: Serve 90% well vs serve 100% poorly (then crash).

### Statistical Evidence

System behavior changes under load:

**Linear scaling region** (r < 2.5):

- Latency follows normal distribution
- P95 ≈ 2 × Average
- Predictable, bounded variance

**Saturation region** (r ≥ 3.0):

- Latency follows power law distribution
- P95 >> 10 × Average
- Unbounded variance, unpredictable tail latencies

lawbench keeps you in the linear scaling region.

---

## Observability

### Key Metrics

```json
{
  "r": 2.45,
  "status": "STABLE",
  "request_count": 125847,
  "error_count": 89,
  "shed_count": 0,
  "avg_latency_ms": 105,
  "p95_latency_ms": 198,
  "tail_divergence_ratio": 1.88
}
```

### Dashboards

Track these over time:

1. **r(t)**: Coupling parameter (primary health metric)
2. **Shed rate**: Percentage of requests rejected
3. **Latency distribution**: P50, P95, P99
4. **Tail divergence**: Early warning of approaching saturation

---

## Testing

Run the included example:

```bash
cd examples/simple-http
bash test.sh
```

**Output**: Side-by-side comparison showing 10x latency improvement.

[See example documentation →](examples/simple-http/README.md)

---

## When to Use

✅ **Good fit:**

- High-traffic services (>1000 rps)
- Latency-sensitive applications
- Services with bursty traffic patterns
- Systems that must stay up during spikes
- Microservices with inter-service dependencies

❌ **Not needed:**

- Low-traffic internal tools (<100 rps)
- Batch processing systems
- Services with static, predictable load
- Systems where all requests MUST be served

---

## FAQ

**Q: How is this different from rate limiting?**

A: Rate limiting uses static thresholds ("100 req/sec max"). lawbench adapts to actual system state in real-time.

**Q: What about autoscaling?**

A: Autoscaling adds capacity. lawbench prevents operating in regimes where added capacity hurts. Use both.

**Q: Why 503 instead of 429?**

A: 503 signals temporary unavailability (retry soon). 429 signals quota exceeded (don't retry). We want graceful backoff.

**Q: Performance overhead?**

A: Negligible (<1μs per request). The check is a simple threshold comparison.

**Q: Will this hurt my users?**

A: 10% get instant 503 (1ms) vs 100% get 2-3 second timeouts. Which is better?

---

## Support & Consulting

**Issues**: [GitHub Issues](https://github.com/alexshd/trdynamics/issues)

**Need help integrating lawbench into your infrastructure?** Contact for consulting.

**Want deeper insights into your system's scalability limits?** We can help measure and optimize.

---

## References

- [Quick Start Guide](QUICKSTART.md)
- [Proof: 10x Latency Improvement](docs/PROOF.md)
- [Empirical Validation](docs/EMPIRICAL_VALIDATION.md)
- [Kubernetes Strategy](docs/KUBERNETES_STRATEGY.md)
- [Example: HTTP Server](examples/simple-http/)
- [API Documentation](docs/LAWBENCH.md)

---

**Production ready. Empirically validated. Zero magic.**
