# Simple HTTP Server: Before & After lawbench

This example demonstrates the difference between a server with and without lawbench protection.

## Quick Start

**Prerequisites**: [See installation guide](INSTALL.md) for detailed setup instructions.

```bash
# Automated comparison test
./test.sh
```

This script runs both examples and shows the difference in a single command.

The script will automatically check for required dependencies (Go, k6, jq, bc, curl) and show installation instructions if anything is missing.

## Files

- `without_lawbench.go` - Naive server (will crash under load)
- `with/with_lawbench.go` - Protected server (graceful degradation)
- `load_test.js` - k6 load test script
- `test.sh` - Automated test runner

**Note**: Both Go files have `package main` so they cannot be compiled together. Run them separately as shown below.

## Run the Comparison

### Test WITHOUT lawbench (The Problem)

```bash
# Terminal 1: Start unprotected server
go run without_lawbench.go

# Terminal 2: Run load test
k6 run --vus 50 --duration 30s load_test.js
```

**Expected result:**

- ❌ 50-60% failure rate
- ❌ P95 latency > 5 seconds
- ❌ Some requests timeout completely
- ❌ System unstable, may need restart

### Test WITH lawbench (The Solution)

```bash
# Terminal 1: Start protected server
go run with/with_lawbench.go

# Terminal 2: Monitor r(t) in real-time
watch -n 0.5 'curl -s http://localhost:8080/lawbench | jq ".r, .status"'

# Terminal 3: Run same load test
k6 run --vus 50 --duration 30s load_test.js
```

**Expected result:**

- ✅ 80-85% success rate (20% intentionally shed)
- ✅ P95 latency < 500ms (fast for successful requests)
- ✅ No timeouts
- ✅ System remains stable

## The Difference

### Without lawbench:

```
Load increases → Latency explodes → Errors cascade → Total failure
Everyone suffers (0% good service)
```

### With lawbench:

```
Load increases → r(t) rises → lawbench sheds 20% → System stabilizes
80% get perfect service, 20% see 503 (controlled rejection)
```

**Key insight**: Better to serve 80% perfectly than to serve 0% during cascade.

## What You'll See

### Without lawbench:

```bash
k6 summary:
    checks.........................: 45% ✓ 900   ✗ 1100
    http_req_duration..............: avg=2.8s p(95)=8.5s
    http_req_failed................: 55%
```

### With lawbench:

```bash
k6 summary:
    checks.........................: 80% ✓ 1600  ✗ 400
    http_req_duration..............: avg=180ms p(95)=420ms
    http_req_failed................: 20% (lawbench protection)

lawbench status:
{
  "r": 2.95,
  "status": "WARNING",
  "governor": {
    "action": "PACING",
    "reason": "Load shedding active (r approaching 3.0)"
  }
}
```

## Monitoring r(t)

While running `with_lawbench.go`, you can watch the system's "heart rate" in real-time:

```bash
# Watch r(t) every 0.5 seconds
watch -n 0.5 'curl -s http://localhost:8080/lawbench | jq "{r, status, action: .governor.action}"'
```

**What you'll see:**

```json
{
  "r": 1.8,
  "status": "STABLE",
  "action": "STABLE"
}

... load increases ...

{
  "r": 2.6,
  "status": "WARNING",
  "action": "WARNING"
}

... load continues ...

{
  "r": 2.95,
  "status": "WARNING",
  "action": "PACING"
}
```

This is the **thermodynamic governor** in action.

## The Physics

The r parameter measures coupling between system components:

```
r = 1.5 + (latency_ms / 100) + (error_rate × 2)
```

- **r < 2.0**: Stable regime (asymptotic behavior)
- **r = 2.8**: Warning (approaching boundary)
- **r = 2.9**: Danger (apply pacing)
- **r ≥ 3.0**: Chaos (emergency shock)

lawbench monitors r(t) continuously and acts **before** cascade failure.

## Requirements

- **Go** 1.21+: https://go.dev/doc/install
- **k6** (load testing): `brew install k6` or https://k6.io/docs/getting-started/installation/
- **jq** (JSON parsing): `brew install jq` or `apt install jq`
- **bc** (calculations): `brew install bc` or `apt install bc`
- **curl** (usually pre-installed)

The test script will check for all dependencies automatically.

## Next Steps

1. Run both examples to see the difference
2. Try different load levels (change VUs in k6)
3. Monitor `/lawbench` endpoint during load
4. Read [README_PRODUCT.md](../../README_PRODUCT.md) for full docs
