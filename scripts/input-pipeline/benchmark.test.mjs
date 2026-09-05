import test from 'node:test';
import assert from 'node:assert/strict';
import { pairedSchedule, percentile, report, validateRecord } from './benchmark.mjs';

test('nearest rank preserves empty, singleton and 1..100 fixtures', () => {
  assert.equal(percentile([], .5), null);
  assert.equal(percentile([7], .95), 7);
  const values = Array.from({ length: 100 }, (_, i) => i + 1);
  assert.equal(percentile(values, .5), 50);
  assert.equal(percentile(values, .95), 95);
  assert.throws(() => percentile([NaN], .5));
});
test('schedule contains both arms and balances reproducible AB/BA independent pairs', () => {
  const schedule = pairedSchedule(30, 42);
  assert.deepEqual(schedule, pairedSchedule(30, 42));
  assert.notDeepEqual(schedule, pairedSchedule(30, 43));
  assert.equal(schedule.filter(p => p.order[0] === 'A').length, 15);
  assert.equal(new Set(schedule.map(p => p.pair)).size, 30);
  assert.ok(schedule.every(p => [...p.order].sort().join('') === 'AB'));
});
const record = (pair, arm) => ({ pair, arm, requests: 1, retries: 0, outcome: 'success', cache_status: 'unreported',
  metrics: { total_us: 100, first_text_us: arm === 'A' ? 50 : 40 } });
test('paired bootstrap excludes neither failures nor missing text from accounting', () => {
  const rows = Array.from({ length: 30 }, (_, i) => [record(i, 'A'), record(i, 'B')]).flat();
  rows[0].outcome = 'timeout'; rows[0].metrics.first_text_us = null; rows[0].retries = 2;
  const a = report(rows, { seed: 42, workload: 'warm', expectedPairs: 30 });
  assert.deepEqual(a, report(rows, { seed: 42, workload: 'warm', expectedPairs: 30 }));
  assert.equal(a.metrics.first_text_us.complete_pairs, 29);
  assert.equal(a.counts.A.timeouts, 1);
  assert.equal(a.counts.A.missing_text, 1);
  assert.equal(a.counts.A.retries, 2);
  assert.deepEqual(a.metrics.first_text_us.p50.ci95, [20, 20]);
  assert.equal(a.decision, 'no-rollout');
  assert.equal(a.prompt_cache_key_default, 'OFF');
});
test('zero captures, missing pair and duplicate arm fail; unknown fields are never exported', () => {
  assert.throws(() => validateRecord({ ...record(0, 'A'), requests: 0 }), /zero_captures/);
  assert.throws(() => report([record(0, 'A')], { seed: 1, workload: 'fresh', expectedPairs: 1 }), /missing_pair/);
  assert.throws(() => report([record(0, 'A'), record(0, 'A')], { seed: 1, workload: 'fresh', expectedPairs: 1 }), /duplicate_arm/);
  assert.ok(!JSON.stringify(validateRecord({ ...record(0, 'A'), prompt: 'SECRET_CANARY' })).includes('SECRET_CANARY'));
});
