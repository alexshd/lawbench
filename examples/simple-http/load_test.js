// k6 load test for before/after comparison
import http from "k6/http";
import { check, sleep } from "k6";

// EXTREME MODE: Push to 300 VUs to trigger cascade failure
export let options = {
  stages: [
    { duration: "10s", target: 100 },  // Ramp to 100 VUs
    { duration: "10s", target: 200 },  // Ramp to 200 VUs  
    { duration: "10s", target: 300 },  // Ramp to 300 VUs (CASCADE ZONE)
    { duration: "15s", target: 300 },  // Hold at 300 VUs (breaking point)
    { duration: "5s", target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_failed: ['rate<0.70'], // Allow up to 70% failure to see the crash
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export default function () {
  // Send order request (with timeout)
  const res = http.get(`${BASE_URL}/api/order`, {
    timeout: "5s",
  });

  check(res, {
    "status 200 (success)": (r) => r.status === 200,
    "status 503 (graceful)": (r) => r.status === 503,
    "not timeout/error": (r) => r.status !== 0,
    "response time < 2s": (r) => r.timings.duration < 2000,
  });

  sleep(0.05); // Aggressive think time (50ms)
}

export function handleSummary(data) {
  const passed = data.metrics.checks.values.passes;
  const failed = data.metrics.checks.values.fails;
  const total = passed + failed;
  const passRate = ((passed / total) * 100).toFixed(2);

  const totalReqs = data.metrics.http_reqs.values.count;
  const failedReqs = data.metrics.http_req_failed
    ? data.metrics.http_req_failed.values.passes
    : 0;
  const successReqs = totalReqs - failedReqs;
  const reqSuccessRate = ((successReqs / totalReqs) * 100).toFixed(2);

  const avgDuration = data.metrics.http_req_duration.values.avg.toFixed(2);
  const p95Duration = data.metrics.http_req_duration.values["p(95)"].toFixed(2);
  const p99Duration =
    data.metrics.http_req_duration && data.metrics.http_req_duration.values["p(99)"]
      ? data.metrics.http_req_duration.values["p(99)"].toFixed(2)
      : "N/A";

  console.log("\n=== LOAD TEST RESULTS ===");
  console.log(`Total Requests:   ${totalReqs}`);
  console.log(`Success Rate:     ${reqSuccessRate}% (${successReqs}/${totalReqs})`);
  console.log(`Check Pass Rate:  ${passRate}% (${passed}/${total})`);
  console.log(`Failed Requests:  ${failedReqs}`);
  console.log(`\nLatency:`);
  console.log(`  Average:        ${avgDuration}ms`);
  console.log(`  P95:            ${p95Duration}ms`);
  console.log(`  P99:            ${p99Duration}ms`);

  return {
    stdout: JSON.stringify(data, null, 2),
  };
}
