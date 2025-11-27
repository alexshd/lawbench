// Example: HTTP server WITHOUT lawbench - will crash under load
package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/api/order", handleOrder)
	http.HandleFunc("/health", handleHealth)

	log.Println("Server starting on :8080 (NO PROTECTION)")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleOrder(w http.ResponseWriter, r *http.Request) {
	// Simulate variable processing time with occasional SLOW queries
	processingTime := time.Duration(rand.Intn(150)) * time.Millisecond

	// 10% chance of SLOW query (simulates database lock, network hiccup)
	if rand.Float64() < 0.10 {
		processingTime = time.Duration(1000+rand.Intn(2000)) * time.Millisecond // 1-3 seconds!
	}

	time.Sleep(processingTime)

	// Simulate occasional errors (10% failure rate under stress)
	if rand.Float64() < 0.10 {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	// Simulate memory allocation (represents object creation, caching, etc.)
	// Under high load, this causes GC pressure
	data := make([]byte, 1024*100) // 100KB per request
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id":  rand.Intn(10000),
		"status":    "confirmed",
		"message":   "Order processed successfully",
		"data_size": len(data),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

/*
PROBLEM DEMONSTRATION:

Run this server, then hit it with load:

    # Terminal 1: Start server
    go run without_lawbench.go

    # Terminal 2: Apply load (k6)
    k6 run --vus 50 --duration 30s load_test.js

WHAT HAPPENS:
- At low load (10 VUs): Works fine
- At medium load (30 VUs): Latency increases
- At high load (50+ VUs): Response times explode
- At extreme load (100 VUs): Complete failure, timeouts

WHY:
- No monitoring of r(t)
- No load shedding when approaching chaos
- System blindly accepts all requests until collapse
- Result: Cascade failure, all users affected

TYPICAL OUTPUT (50 VUs):
    checks.........................: 45.00% ✓ 900  ✗ 1100
    http_req_duration..............: avg=2.5s   p(95)=8.2s
    http_req_failed................: 55.00%

↑ More than HALF of requests fail. This is cascade failure.
*/
