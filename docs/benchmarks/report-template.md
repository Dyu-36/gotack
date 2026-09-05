# Input Pipeline Benchmark Report Template

**Workload:** {workload}
**Iterations:** {iterations}
**Seed:** {seed}
**Fake Provider:** {fake_provider}
**Timestamp:** {timestamp}

## Summary

| Metric | Value |
|--------|-------|
| p50    | {p50}ms |
| p95    | {p95}ms |
| mean   | {mean}ms |
| min    | {min}ms |
| max    | {max}ms |

## Breakdown

| Phase | p50 (μs) | p95 (μs) |
|-------|----------|----------|
| ready_wait | {ready_wait_p50} | {ready_wait_p95} |
| mcp_wait | {mcp_wait_p50} | {mcp_wait_p95} |
| prompt_prepare | {prompt_prepare_p50} | {prompt_prepare_p95} |
| request_encode | {request_encode_p50} | {request_encode_p95} |
| request_write_to_first_byte | {rwtb_p50} | {rwtb_p95} |
| first_byte_to_first_sse | {fbfs_p50} | {fbfs_p95} |
| stream | {stream_p50} | {stream_p95} |
| summarize | {sum_p50} | {sum_p95} |

## Cache Status Distribution

| Status | Count | Percentage |
|--------|-------|------------|
| hit | {hit_count} | {hit_pct}% |
| miss | {miss_count} | {miss_pct}% |
| unreported | {unrep_count} | {unrep_pct}% |

## Methodology

- Paired randomization: each iteration assigned to control (A) or treatment (B)
- Duration measured from request start to run_complete
- Cache status tracked as hit/miss/unreported
- Seed ensures reproducibility across runs
- Bootstrap 95% CI computed over paired differences

## Rollout Gate Criteria

- warm visible-TTFT p50 improvement >= 10%
- 95% CI does not cross 0
- p95/full-turn not worse than 5%
- error/retry rate not increased
- no cross-session contamination

## Raw Artifacts

- JSON: `bench-{workload}-{timestamp}.json`
- Report: `bench-{workload}-{timestamp}.md`
