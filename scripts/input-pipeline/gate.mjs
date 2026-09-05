// Phase 0A acceptance rules. No dependencies and no provider content in errors.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

export const requiredTests = Object.freeze([
  'TestHarnessNegativeControls',
  'TestE2EInputPipelineFreshTurn',
  'TestE2ERetryBehavior',
  'TestE2EToolLoopAndRestart',
  'TestE2ERejectMalformedProvider',
]);
export const testPackage = 'github.com/Dyu-36/gotack/e2e/inputpipeline';
export class GateError extends Error {
  constructor(code) { super(`input-pipeline: ${code}`); this.code = code; }
}
export function check(ok, code) { if (!ok) throw new GateError(code); }
export function requireWindows(platform) { check(platform === 'win32', 'required_platform_windows'); }
export function regularFile(filename, code = 'file_missing') {
  try { check(fs.lstatSync(filename).isFile(), code); }
  catch (error) { if (error instanceof GateError) throw error; throw new GateError(code); }
}
export function sha256(filename) {
  regularFile(filename);
  return crypto.createHash('sha256').update(fs.readFileSync(filename)).digest('hex');
}
export function readJSON(filename) {
  regularFile(filename);
  check(fs.statSync(filename).size <= 1024 * 1024, 'json_too_large');
  try { return JSON.parse(fs.readFileSync(filename, 'utf8')); }
  catch { throw new GateError('json_invalid'); }
}
export function recipe(root) {
  const patchDir = path.join(root, 'third_party', 'patches');
  const manifestPath = path.join(patchDir, 'manifest.json');
  const manifest = readJSON(manifestPath);
  check(manifest?.schema_version === 1, 'manifest_version');
  check(Array.isArray(manifest.compatibility) && manifest.compatibility.length > 0 &&
    Array.isArray(manifest.input_pipeline), 'manifest_phases');
  const patches = [];
  const names = new Set();
  for (const phase of ['compatibility', 'input_pipeline']) {
    for (const name of manifest[phase]) {
      check(typeof name === 'string' && /^[a-z0-9][a-z0-9._-]*\.patch$/.test(name), 'patch_path');
      check(!names.has(name), 'patch_duplicate');
      names.add(name);
      patches.push({ phase, path: name, sha256: sha256(path.join(patchDir, name)) });
    }
  }
  const actual = fs.readdirSync(patchDir, { recursive: true }).filter(p => p.endsWith('.patch'));
  check(actual.length === names.size && actual.every(p => names.has(p)), 'patch_inventory');
  const pinPath = path.join(root, '.tack-pin');
  regularFile(pinPath);
  const pin = fs.readFileSync(pinPath, 'utf8').trim();
  check(/^[a-f0-9]{40}$/.test(pin), 'pin_invalid');
  const inputs = {};
  for (const file of [
    'scripts/apply-crush-patches.ps1', 'scripts/harden-crush-for-tack.ps1',
    'scripts/test-input-pipeline-e2e.ps1', 'scripts/input-pipeline/gate.mjs',
    'scripts/input-pipeline/run.mjs',
  ]) inputs[file] = sha256(path.join(root, file));
  return { crush_pin: pin, manifest_sha256: sha256(manifestPath), inputs, patches };
}
export function patchSequence(manifest, skipInputPipeline = false) {
  return [
    ...manifest.compatibility.map(name => `compatibility:${name}`),
    'hardening',
    ...(skipInputPipeline ? [] : manifest.input_pipeline.map(name => `input_pipeline:${name}`)),
  ];
}
export function parseBuildInfo(text) {
  const info = { fantasy: null, goos: '', goarch: '', cgo_enabled: '', trimpath: '' };
  let dependency = '';
  for (const line of text.split(/\r?\n/)) {
    const fields = line.trim().split(/\s+/);
    if (fields[0] === 'dep') {
      dependency = fields[1];
      if (dependency === 'charm.land/fantasy') {
        check(fields.length === 4 && !info.fantasy, 'fantasy_build_info');
        info.fantasy = { path: fields[1], version: fields[2], sum: fields[3] };
      }
    } else if (fields[0] === '=>') {
      check(dependency !== 'charm.land/fantasy', 'fantasy_replace_forbidden');
    } else if (fields[0] === 'build') {
      const pair = fields.slice(1).join(' ').split('=');
      if (pair[0] === 'GOOS') info.goos = pair[1];
      if (pair[0] === 'GOARCH') info.goarch = pair[1];
      if (pair[0] === 'CGO_ENABLED') info.cgo_enabled = pair[1];
      if (pair[0] === '-trimpath') info.trimpath = pair[1];
    }
  }
  check(info.fantasy && /^v\S+$/.test(info.fantasy.version) &&
    /^h1:[A-Za-z0-9+/=]+$/.test(info.fantasy.sum), 'fantasy_build_info');
  check(info.goos === 'windows' && info.goarch === 'amd64' &&
    /^[01]$/.test(info.cgo_enabled) && info.trimpath === 'true', 'binary_build_settings');
  return info;
}
export function verifyProvenance(root, binary, filename, commit, buildInfo) {
  check(path.isAbsolute(binary || '') && path.isAbsolute(filename || ''), 'explicit_absolute_artifacts_required');
  regularFile(binary, 'binary_missing');
  const p = readJSON(filename);
  check(p?.schema_version === 1 && p.input_pipeline_skipped === false, 'provenance_version_or_skipped');
  check(/^[a-f0-9]{40}$/.test(commit) && p.gotack_commit === commit, 'gotack_commit_mismatch');
  check(JSON.stringify(p.recipe) === JSON.stringify(recipe(root)), 'recipe_mismatch');
  check(typeof p.binary?.path === 'string' && path.isAbsolute(p.binary.path), 'binary_path_mismatch');
  regularFile(p.binary.path, 'binary_missing');
  check(fs.realpathSync(binary) === fs.realpathSync(p.binary.path), 'binary_path_mismatch');
  check(p.binary.sha256 === sha256(binary), 'binary_hash_mismatch');
  check(JSON.stringify(p.build_info) === JSON.stringify(buildInfo), 'binary_build_info_mismatch');
  check(JSON.stringify(p.build_command) === JSON.stringify([
    'go', 'build', '-mod=readonly', '-trimpath', '-o', binary, '.',
  ]), 'build_command_mismatch');
  return p;
}
export function verifyTestJSON(text, exitCode) {
  check(exitCode === 0, 'test_process_failed');
  const states = new Map();
  let packagePassed = false;
  for (const line of text.split(/\r?\n/)) {
    if (!line.trim()) continue;
    let event;
    try { event = JSON.parse(line); } catch { throw new GateError('test_json_invalid'); }
    check(event && typeof event === 'object', 'test_json_invalid');
    check(event.Action !== 'skip', 'unexpected_skip');
    check(event.Action !== 'fail', 'test_failed');
    if (event.Package !== testPackage) continue;
    if (event.Action === 'pass' && !event.Test) packagePassed = true;
    if (!requiredTests.includes(event.Test)) continue;
    const state = states.get(event.Test);
    if (event.Action === 'run') {
      check(!state, 'duplicate_required_test');
      states.set(event.Test, 'run');
    } else if (event.Action === 'pass') {
      check(state === 'run', 'required_test_not_run');
      states.set(event.Test, 'pass');
    }
  }
  check(packagePassed, 'package_not_passed');
  check(requiredTests.every(name => states.get(name) === 'pass'), 'required_test_missing');
  return { status: 'PASS', required_tests: requiredTests.length, unexpected_skips: 0 };
}
