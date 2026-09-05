// Statistics consume observed engine records only. Randomness selects arm order
// and bootstrap samples; it never manufactures a latency observation.
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { check } from './gate.mjs';

export const workloads = Object.freeze(['fresh', 'warm', 'long', 'near-compaction', 'synthetic-mcp']);
export function random(seed) {
  check(Number.isInteger(seed) && seed >= 0 && seed <= 0xffffffff, 'benchmark_seed_invalid');
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let n = Math.imul(state ^ (state >>> 15), 1 | state);
    n ^= n + Math.imul(n ^ (n >>> 7), 61 | n);
    return ((n ^ (n >>> 14)) >>> 0) / 4294967296;
  };
}
export function pairedSchedule(pairs, seed) {
  check(Number.isInteger(pairs) && pairs >= 1 && pairs <= 10000, 'benchmark_pairs_invalid');
  const rng = random(seed);
  // Balance AB/BA before shuffling independent pair clusters.
  const orders = Array.from({ length: pairs }, (_, i) => i % 2 ? ['B', 'A'] : ['A', 'B']);
  for (let i = orders.length - 1; i > 0; i--) {
    const j = Math.floor(rng() * (i + 1));
    [orders[i], orders[j]] = [orders[j], orders[i]];
  }
  return orders.map((order, pair) => ({ pair, order }));
}
export function percentile(values, p) {
  check(p > 0 && p <= 1 && values.every(v => Number.isFinite(v) && v >= 0), 'benchmark_sample_invalid');
  if (!values.length) return null;
  return [...values].sort((a, b) => a - b)[Math.ceil(p * values.length) - 1];
}
const metricNames = Object.freeze(['local_preparation_us', 'http_ttfb_us', 'first_reasoning_us',
  'first_tool_us', 'first_text_us', 'total_us', 'summarize_us']);
export function validateRecord(record) {
  check(record && Number.isInteger(record.pair) && record.pair >= 0 && ['A', 'B'].includes(record.arm), 'benchmark_pair_invalid');
  check(['success', 'error', 'timeout', 'cancelled'].includes(record.outcome), 'benchmark_outcome_invalid');
  check(Number.isInteger(record.requests) && record.requests > 0, 'benchmark_zero_captures');
  check(Number.isInteger(record.retries) && record.retries >= 0, 'benchmark_retries_invalid');
  check(['hit', 'miss', 'unreported'].includes(record.cache_status), 'benchmark_cache_invalid');
  check(record.metrics && typeof record.metrics === 'object', 'benchmark_metrics_missing');
  for (const [key, value] of Object.entries(record.metrics)) {
    check(metricNames.includes(key) && (value === null || Number.isFinite(value) && value >= 0), 'benchmark_metric_invalid');
  }
  check(Number.isFinite(record.metrics.total_us) && record.metrics.total_us >= 0, 'benchmark_total_missing');
  // Whitelist projection: no request bodies, session IDs, endpoint or errors.
  return { pair: record.pair, arm: record.arm, outcome: record.outcome, requests: record.requests,
    retries: record.retries, cache_status: record.cache_status,
    metrics: Object.fromEntries(metricNames.map(k => [k, record.metrics[k] ?? null])) };
}
export async function collectPairs({ pairs, seed, runArm }) {
  const records = [];
  for (const { pair, order } of pairedSchedule(pairs, seed)) {
    for (const arm of order) {
      // The executable driver creates a new profile/database for every call.
      // Both arms receive the same pair fixture, with only cache treatment changed.
      const record = validateRecord({ ...await runArm({ pair, arm }), pair, arm });
      records.push(record);
    }
  }
  return records;
}
function change(pairs, metric, p) {
  const a = percentile(pairs.map(pair => pair.A.metrics[metric]), p);
  const b = percentile(pairs.map(pair => pair.B.metrics[metric]), p);
  return a > 0 ? 100 * (a - b) / a : null;
}
export function bootstrap(pairs, metric, p, seed, resamples = 10000) {
  check(Number.isInteger(resamples) && resamples >= 10000, 'benchmark_bootstrap_too_small');
  if (!pairs.length) return null;
  const rng = random(seed), estimates = [];
  for (let n = 0; n < resamples; n++) {
    const sample = Array.from({ length: pairs.length }, () => pairs[Math.floor(rng() * pairs.length)]);
    const estimate = change(sample, metric, p);
    if (estimate !== null) estimates.push(estimate);
  }
  if (estimates.length !== resamples) return null;
  estimates.sort((a, b) => a - b);
  return [estimates[Math.ceil(.025 * resamples) - 1], estimates[Math.ceil(.975 * resamples) - 1]];
}
export function report(input, { seed, workload, synthetic = true, expectedPairs }) {
  check(workloads.includes(workload), 'benchmark_workload_invalid');
  const records = input.map(validateRecord), grouped = new Map();
  for (const record of records) {
    const pair = grouped.get(record.pair) || {};
    check(!pair[record.arm], 'benchmark_duplicate_arm');
    pair[record.arm] = record;
    grouped.set(record.pair, pair);
  }
  check(grouped.size === expectedPairs && [...grouped.values()].every(p => p.A && p.B), 'benchmark_missing_pair');
  const pairs = [...grouped.values()], metrics = {};
  for (const metric of metricNames) {
    const complete = pairs.filter(p => ['A', 'B'].every(arm => p[arm].outcome === 'success' && p[arm].metrics[metric] !== null));
    metrics[metric] = { complete_pairs: complete.length, missing_or_censored_pairs: pairs.length - complete.length };
    for (const [label, p] of [['p50', .5], ['p95', .95]]) {
      metrics[metric][label] = {
        A: percentile(complete.map(v => v.A.metrics[metric]), p),
        B: percentile(complete.map(v => v.B.metrics[metric]), p),
        improvement_percent: complete.length ? change(complete, metric, p) : null,
        ci95: bootstrap(complete, metric, p, seed),
      };
    }
  }
  const counts = Object.fromEntries(['A', 'B'].map(arm => {
    const values = records.filter(r => r.arm === arm);
    return [arm, { errors: values.filter(r => r.outcome === 'error').length,
      timeouts: values.filter(r => r.outcome === 'timeout').length,
      cancelled: values.filter(r => r.outcome === 'cancelled').length,
      missing_text: values.filter(r => r.metrics.first_text_us === null).length,
      retries: values.reduce((sum, r) => sum + r.retries, 0) }];
  }));
  // No automatic rollout from synthetic data or unapproved endpoint/budget.
  return { schema_version: 1, synthetic, workload, seed, independent_pairs: pairs.length,
    percentile_method: 'nearest-rank', bootstrap_resamples: 10000, bootstrap_unit: 'session-pair',
    cache_state: 'provider-cache-reset-unproven', metrics, counts,
    decision: 'no-rollout', prompt_cache_key_default: 'OFF',
    reason: synthetic ? 'synthetic_correctness_only' : 'inconclusive_pending_preregistered_live_acceptance', records };
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const [input, output, workload, pairText, seedText] = process.argv.slice(2);
    check(input && output, 'benchmark_artifacts_required');
    const records = fs.readFileSync(input, 'utf8').trim().split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
    const result = report(records, { workload, expectedPairs: Number(pairText), seed: Number(seedText), synthetic: true });
    fs.writeFileSync(output, `${JSON.stringify(result, null, 2)}\n`, { flag: 'wx' });
    console.log(`Benchmark: ${result.independent_pairs} independent pairs; synthetic=true; no-rollout; cache OFF.`);
  } catch (error) { console.error(error?.code || 'benchmark_artifact_invalid'); process.exitCode = 1; }
}
