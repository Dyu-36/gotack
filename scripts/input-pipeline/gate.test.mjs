import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  recipe, sha256, patchSequence, parseBuildInfo, verifyProvenance,
  verifyTestJSON, requiredTests, testPackage, requireWindows,
} from './gate.mjs';
import { native, checked, newArtifactDir, buildCandidate, gitEnvironment } from './run.mjs';

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gotack-gate-unit-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const write = (name, content) => {
    const file = path.join(root, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content);
    return file;
  };
  write('.tack-pin', 'a'.repeat(40));
  const manifest = { schema_version: 1, compatibility: ['a.patch'], input_pipeline: ['z.patch'] };
  write('third_party/patches/manifest.json', JSON.stringify(manifest));
  write('third_party/patches/a.patch', 'compatibility\n');
  write('third_party/patches/z.patch', 'phase\n');
  for (const file of ['apply-crush-patches.ps1', 'harden-crush-for-tack.ps1', 'test-input-pipeline-e2e.ps1',
    'input-pipeline/gate.mjs', 'input-pipeline/run.mjs']) write(`scripts/${file}`, 'fixture\n');
  const binary = write('candidate.exe', 'synthetic binary');
  const buildText = '\tdep\tcharm.land/fantasy\tv0.41.3\th1:abcd=\n' +
    '\tbuild\tGOOS=windows\n\tbuild\tGOARCH=amd64\n\tbuild\tCGO_ENABLED=0\n\tbuild\t-trimpath=true\n';
  const info = parseBuildInfo(buildText);
  const commit = 'b'.repeat(40);
  const p = { schema_version: 1, input_pipeline_skipped: false, gotack_commit: commit,
    recipe: recipe(root), binary: { path: binary, sha256: sha256(binary) }, build_info: info,
    build_command: ['go', 'build', '-mod=readonly', '-trimpath', '-o', binary, '.'] };
  const provenance = write('provenance.json', JSON.stringify(p));
  return { root, write, manifest, binary, provenance, info, buildText, commit, p };
}
const rejects = (fn, code) => assert.throws(fn, error => error.code === code);

function passStream() {
  const events = [];
  for (const Test of requiredTests) events.push(
    { Action: 'run', Package: testPackage, Test }, { Action: 'pass', Package: testPackage, Test });
  events.push({ Action: 'pass', Package: testPackage });
  return events;
}
const jsonl = events => events.map(e => JSON.stringify(e)).join('\n') + '\n';

test('required tests need both RUN and PASS, plus package success', () => {
  assert.equal(verifyTestJSON(jsonl(passStream()), 0).required_tests, requiredTests.length);
  rejects(() => verifyTestJSON('', 0), 'package_not_passed');
  rejects(() => verifyTestJSON(jsonl(passStream().slice(2)), 0), 'required_test_missing');
  rejects(() => verifyTestJSON(jsonl(passStream().slice(1)), 0), 'required_test_not_run');
  rejects(() => verifyTestJSON(jsonl(passStream()), 1), 'test_process_failed');
  rejects(() => verifyTestJSON('{broken', 0), 'test_json_invalid');
  rejects(() => verifyTestJSON(jsonl(passStream().slice(0, -1)), 0), 'package_not_passed');
});
test('unexpected skips, failures and duplicate runs cannot pass', () => {
  for (const [action, code] of [['skip', 'unexpected_skip'], ['fail', 'test_failed']]) {
    rejects(() => verifyTestJSON(jsonl([...passStream(), { Action: action, Test: 'AnotherTest' }]), 0), code);
  }
  rejects(() => verifyTestJSON(jsonl([...passStream(), passStream()[0]]), 0), 'duplicate_required_test');
});
test('manifest has an explicit hardening boundary and a real skip flag', t => {
  const f = fixture(t);
  assert.deepEqual(patchSequence(f.manifest), ['compatibility:a.patch', 'hardening', 'input_pipeline:z.patch']);
  assert.deepEqual(patchSequence(f.manifest, true), ['compatibility:a.patch', 'hardening']);
  assert.equal(recipe(f.root).patches.length, 2);
  f.write('third_party/patches/unlisted.patch', 'not declared');
  rejects(() => recipe(f.root), 'patch_inventory');
});
test('manifest rejects missing, duplicate, and escaping patches', t => {
  const f = fixture(t);
  f.manifest.input_pipeline = ['a.patch'];
  f.write('third_party/patches/manifest.json', JSON.stringify(f.manifest));
  rejects(() => recipe(f.root), 'patch_duplicate');
  f.manifest.input_pipeline = ['../escape.patch'];
  f.write('third_party/patches/manifest.json', JSON.stringify(f.manifest));
  rejects(() => recipe(f.root), 'patch_path');
  f.manifest.input_pipeline = ['missing.patch'];
  f.write('third_party/patches/manifest.json', JSON.stringify(f.manifest));
  rejects(() => recipe(f.root), 'file_missing');
});
test('explicit binary and complete matching provenance are mandatory', t => {
  const f = fixture(t);
  assert.equal(verifyProvenance(f.root, f.binary, f.provenance, f.commit, f.info).gotack_commit, f.commit);
  rejects(() => verifyProvenance(f.root, 'candidate.exe', f.provenance, f.commit, f.info), 'explicit_absolute_artifacts_required');
  rejects(() => verifyProvenance(f.root, path.join(f.root, 'missing.exe'), f.provenance, f.commit, f.info), 'binary_missing');
  rejects(() => verifyProvenance(f.root, f.binary, f.provenance, 'c'.repeat(40), f.info), 'gotack_commit_mismatch');
  f.write('candidate.exe', 'changed binary');
  rejects(() => verifyProvenance(f.root, f.binary, f.provenance, f.commit, f.info), 'binary_hash_mismatch');
});
test('provenance rejects skipped phases, changed recipes and build settings', t => {
  const f = fixture(t);
  f.p.input_pipeline_skipped = true;
  f.write('provenance.json', JSON.stringify(f.p));
  rejects(() => verifyProvenance(f.root, f.binary, f.provenance, f.commit, f.info), 'provenance_version_or_skipped');
  f.p.input_pipeline_skipped = false;
  f.write('provenance.json', JSON.stringify(f.p));
  rejects(() => verifyProvenance(f.root, f.binary, f.provenance, f.commit, { ...f.info, goos: 'linux' }), 'binary_build_info_mismatch');
  f.write('scripts/harden-crush-for-tack.ps1', 'changed hardening');
  rejects(() => verifyProvenance(f.root, f.binary, f.provenance, f.commit, f.info), 'recipe_mismatch');
});
test('build metadata rejects local Fantasy replacement and wrong platform', t => {
  const f = fixture(t);
  rejects(() => parseBuildInfo(f.buildText.replace('GOOS=windows', 'GOOS=linux')), 'binary_build_settings');
  rejects(() => parseBuildInfo('\tdep\tcharm.land/fantasy\tv0.41.3\th1:abcd=\n\t=>\t./local-fantasy\n'), 'fantasy_replace_forbidden');
});
test('unique temp directories use RUNNER_TEMP or an OS fallback', t => {
  const f = fixture(t);
  const a = newArtifactDir({ RUNNER_TEMP: f.root });
  const b = newArtifactDir({ RUNNER_TEMP: f.root });
  assert.notEqual(a, b);
  const c = newArtifactDir({});
  t.after(() => fs.rmSync(c, { recursive: true, force: true }));
  assert.ok(path.isAbsolute(c));
});
test('native command failure, missing command and timeout are checked', async () => {
  await assert.rejects(() => checked(native, process.execPath, ['-e', 'process.exit(7)'], {}, 'native_failed'),
    error => error.code === 'native_failed');
  await assert.rejects(() => checked(native, path.join(os.tmpdir(), 'gotack-no-such-program-unique'), [], {}, 'native_failed'),
    error => error.code === 'native_start_failed');
  await assert.rejects(() => checked(native, process.execPath, ['-e', 'setInterval(()=>{},1000)'], { timeout: 100 }, 'native_failed'),
    error => error.code === 'native_timeout');
});
test('build orchestration checks the pin, orders replay before root build and records metadata', async t => {
  const f = fixture(t);
  const calls = [];
  const artifacts = path.join(f.root, 'artifacts');
  fs.mkdirSync(artifacts);
  const ps = f.write('pwsh.exe', 'fixture');
  const run = async (file, args, opts) => {
    calls.push({ file, args, opts });
    let stdout = '';
    if (file === 'git' && args.includes('rev-parse')) stdout = args[1] === f.root ? f.commit : 'a'.repeat(40);
    if (file === 'go' && args[0] === 'build') fs.writeFileSync(args[4], 'built fixture');
    if (file === 'go' && args[0] === 'version') stdout = f.buildText;
    if (file === 'go' && args[0] === 'list') stdout = JSON.stringify({
      Path: 'charm.land/fantasy', Version: 'v0.41.3', Sum: 'h1:abcd=',
    });
    return { status: 0, stdout, failure: '' };
  };
  const result = await buildCandidate(f.root, artifacts, ps, run);
  const build = calls.findIndex(c => c.file === 'go' && c.args[0] === 'build');
  assert.ok(calls.findIndex(c => c.file === ps) < build);
  assert.equal(calls[build].args.at(-1), '.');
  assert.ok(calls.some(c => c.args.includes('--detach')));
  assert.equal(JSON.parse(fs.readFileSync(result.provenance)).binary.sha256, sha256(result.binary));
  assert.ok(calls.every(c => !c.args.some(a => String(a).includes('third_party/crush'))));
});
test('failed clone/fetch must stop before patching or build', async t => {
  const f = fixture(t);
  const artifacts = path.join(f.root, 'artifacts');
  fs.mkdirSync(artifacts);
  const ps = f.write('pwsh.exe', 'fixture');
  let buildCalled = false;
  const run = async (file, args) => {
    if (file === 'go') buildCalled = true;
    return { status: args.includes('fetch') ? 9 : 0,
      stdout: args.includes('rev-parse') ? f.commit : '', failure: '' };
  };
  await assert.rejects(() => buildCandidate(f.root, artifacts, ps, run), error => error.code === 'clean_pin_replay_failed');
  assert.equal(buildCalled, false);
});

test('required platform and Git directory isolation fail closed', () => {
  requireWindows('win32');
  for (const platform of ['linux', 'darwin', '']) rejects(() => requireWindows(platform), 'required_platform_windows');
  const source = { PATH: 'tools', GIT_DIR: '/owner/repo/.git', GIT_WORK_TREE: '/owner/repo', GIT_CONFIG_COUNT: '1' };
  const clean = gitEnvironment(source);
  assert.deepEqual(clean, { PATH: 'tools', GIT_TERMINAL_PROMPT: '0' });
  assert.equal(source.GIT_DIR, '/owner/repo/.git');
});
