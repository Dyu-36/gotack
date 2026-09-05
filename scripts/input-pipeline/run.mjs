// Windows executable gate. Every subprocess has a deadline and checked status.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn, spawnSync } from 'node:child_process';
import {
  GateError, check, recipe, sha256, parseBuildInfo, regularFile,
  verifyProvenance, verifyTestJSON, requireWindows,
} from './gate.mjs';

const defaultRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
export function native(file, args, options = {}) {
  const { cwd, env = process.env, timeout = 120_000, maxBytes = 32 * 1024 * 1024 } = options;
  return new Promise(resolve => {
    let stdout = '', stderr = '', failure = '', settled = false, cleanupTimer;
    const child = spawn(file, args, {
      cwd, env, shell: false, windowsHide: true,
      detached: process.platform !== 'win32', stdio: ['ignore', 'pipe', 'pipe'],
    });
    const finish = status => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      clearTimeout(cleanupTimer);
      resolve({ status, stdout, stderr, failure });
    };
    const killOwned = reason => {
      if (failure) return;
      failure = reason;
      cleanupTimer = setTimeout(() => {
        failure = 'native_cleanup_unverified';
        child.stdout.destroy(); child.stderr.destroy();
        finish(null);
      }, 12_000);
      if (child.pid && child.exitCode === null && child.signalCode === null) {
        if (process.platform === 'win32') {
          const systemRoot = process.env.SystemRoot || process.env.SYSTEMROOT;
          if (systemRoot) spawnSync(path.join(systemRoot, 'System32', 'taskkill.exe'),
            ['/PID', String(child.pid), '/T', '/F'], { timeout: 10_000, windowsHide: true, stdio: 'ignore' });
          child.kill();
        } else {
          try { process.kill(-child.pid, 'SIGKILL'); } catch { child.kill('SIGKILL'); }
        }
      }
    };
    const timer = setTimeout(() => killOwned('native_timeout'), timeout);
    child.stdout.on('data', chunk => {
      if (stdout.length + chunk.length > maxBytes) killOwned('native_output_limit');
      else stdout += chunk.toString('utf8');
    });
    child.stderr.on('data', chunk => {
      // Never echo rejected input, credentials, raw provider errors, or tool output.
      if (stderr.length + chunk.length <= 64 * 1024) stderr += chunk.toString('utf8');
    });
    child.once('error', () => { failure = 'native_start_failed'; finish(null); });
    child.once('close', code => finish(code));
  });
}
export async function checked(run, file, args, options, code) {
  const result = await run(file, args, options);
  check(!result.failure && result.status === 0, result.failure || code);
  return result.stdout.trim();
}
export function gitEnvironment(env = process.env) {
  const clean = { ...env };
  for (const key of Object.keys(clean)) if (key.toUpperCase().startsWith('GIT_')) delete clean[key];
  clean.GIT_TERMINAL_PROMPT = '0';
  return clean;
}
export async function currentCommit(root, run) {
  const options = { env: gitEnvironment() };
  const commit = await checked(run, 'git', ['-C', root, 'rev-parse', 'HEAD'], options, 'gotack_commit_unavailable');
  check(/^[a-f0-9]{40}$/.test(commit), 'gotack_commit_invalid');
  // Ignored owner engine trees are neither read nor changed. Tracked input
  // edits cannot be attributed to HEAD, so reject them instead of guessing.
  await checked(run, 'git', ['-C', root, 'diff', '--exit-code', 'HEAD', '--',
    '.tack-pin', 'third_party/patches', 'scripts', 'e2e/inputpipeline'], options, 'uncommitted_gate_inputs');
  const untracked = await checked(run, 'git', ['-C', root, 'ls-files', '--others', '--exclude-standard', '--',
    'third_party/patches', 'scripts/input-pipeline', 'e2e/inputpipeline'], options, 'input_inventory_unavailable');
  check(untracked === '', 'untracked_gate_inputs');
  return commit;
}
export function newArtifactDir(env = process.env) {
  const base = env.RUNNER_TEMP || os.tmpdir();
  check(path.isAbsolute(base) && fs.existsSync(base) && fs.statSync(base).isDirectory(), 'temp_root_invalid');
  return fs.mkdtempSync(path.join(base, 'gotack-input-pipeline-'));
}
export async function buildCandidate(root, artifacts, powershell, run = native) {
  check(path.isAbsolute(powershell || ''), 'powershell_absolute_path_required');
  regularFile(powershell, 'powershell_missing');
  const commit = await currentCommit(root, run);
  const inputs = recipe(root);
  const source = path.join(artifacts, 'crush-source');
  fs.mkdirSync(source); // Must be new; never reuse the owner's ignored checkout.
  const git = async (...args) => checked(run, 'git', ['-C', source, ...args],
    { timeout: 180_000, env: gitEnvironment() }, 'clean_pin_replay_failed');
  await git('init', '--quiet', '--template=');
  await git('config', 'core.hooksPath', path.join(artifacts, 'no-hooks'));
  await git('config', 'core.autocrlf', 'false');
  await git('remote', 'add', 'origin', 'https://github.com/charmbracelet/crush.git');
  await git('fetch', '--depth=1', 'origin', inputs.crush_pin);
  await git('-c', 'core.autocrlf=false', 'checkout', '--detach', 'FETCH_HEAD');
  check(await git('rev-parse', 'HEAD') === inputs.crush_pin, 'crush_pin_mismatch');
  check(await git('status', '--porcelain', '--untracked-files=all') === '', 'clean_pin_dirty');
  await checked(run, powershell, ['-NoLogo', '-NoProfile', '-NonInteractive', '-File',
    path.join(root, 'scripts/apply-crush-patches.ps1'), '-CrushDir', source],
  { cwd: root, timeout: 180_000, env: gitEnvironment() }, 'patch_replay_failed');
  check(JSON.stringify(inputs) === JSON.stringify(recipe(root)), 'inputs_changed_during_replay');
  const binary = path.join(artifacts, 'tack-engine-e2e.exe');
  const buildArgs = ['build', '-mod=readonly', '-trimpath', '-o', binary, '.'];
  // Do not change the owner's global toolchain or Go settings.
  const env = { ...gitEnvironment(), GOFLAGS: '', GOWORK: 'off' };
  await checked(run, 'go', buildArgs, { cwd: source, env, timeout: 900_000 }, 'engine_build_failed');
  regularFile(binary, 'built_binary_missing');
  const buildInfo = parseBuildInfo(await checked(run, 'go', ['version', '-m', binary],
    { env }, 'build_info_unavailable'));
  const dependency = JSON.parse(await checked(run, 'go', ['list', '-mod=readonly', '-m', '-json', 'charm.land/fantasy'],
    { cwd: source, env }, 'fantasy_dependency_unavailable'));
  check(!dependency.Replace && dependency.Path === buildInfo.fantasy.path &&
    dependency.Version === buildInfo.fantasy.version && dependency.Sum === buildInfo.fantasy.sum,
  'fantasy_dependency_mismatch');
  check(await currentCommit(root, run) === commit &&
    JSON.stringify(inputs) === JSON.stringify(recipe(root)), 'inputs_changed_during_build');
  const p = {
    schema_version: 1, gotack_commit: commit, recipe: inputs, input_pipeline_skipped: false,
    binary: { path: binary, sha256: sha256(binary) }, build_info: buildInfo,
    build_command: ['go', ...buildArgs],
  };
  const provenance = path.join(artifacts, 'provenance.json');
  fs.writeFileSync(provenance, `${JSON.stringify(p, null, 2)}\n`, { encoding: 'utf8', flag: 'wx' });
  verifyProvenance(root, binary, provenance, commit, buildInfo);
  return { binary, provenance };
}
export async function runGate(options, run = native) {
  requireWindows(process.platform);
  const root = defaultRoot;
  check(!options.verifyOnly || (options.skipBuild && !options.buildOnly), 'verify_requires_explicit_binary');
  const artifacts = options.verifyOnly ? '' : newArtifactDir();
  if (artifacts) console.log(`Artifacts: ${artifacts}`);
  let binary = options.binary || '', provenance = options.provenance || '';
  if (options.skipBuild) {
    check(!options.buildOnly, 'conflicting_build_flags');
    const commit = await currentCommit(root, run);
    check(path.isAbsolute(binary) && path.isAbsolute(provenance), 'explicit_absolute_artifacts_required');
    regularFile(binary, 'binary_missing');
    const info = parseBuildInfo(await checked(run, 'go', ['version', '-m', binary], {}, 'build_info_unavailable'));
    verifyProvenance(root, binary, provenance, commit, info);
  } else {
    check(!binary && !provenance, 'binary_arguments_require_skip_build');
    ({ binary, provenance } = await buildCandidate(root, artifacts, options.powershell, run));
  }
  if (options.verifyOnly) return;
  console.log(`Binary: ${binary}\nProvenance: ${provenance}`);
  if (options.buildOnly) {
    console.log('BUILD VERIFIED; E2E NOT RUN');
    return;
  }
  const result = await run('go', ['test', '-json', '-tags=e2e', './e2e/inputpipeline', '-count=1', '-timeout=10m'], {
    cwd: root, timeout: 660_000,
    env: { ...gitEnvironment(), GOFLAGS: '', GOWORK: 'off', TACK_ENGINE_BINARY: binary,
      TACK_ENGINE_PROVENANCE: provenance, TACK_E2E_REPO_ROOT: root, TACK_E2E_NODE: process.execPath },
  });
  fs.writeFileSync(path.join(artifacts, 'tests.jsonl'), result.stdout, 'utf8');
  check(!result.failure, result.failure || 'test_process_failed');
  const summary = verifyTestJSON(result.stdout, result.status);
  fs.writeFileSync(path.join(artifacts, 'result.json'), `${JSON.stringify(summary, null, 2)}\n`, 'utf8');
  console.log(`E2E PASS: ${summary.required_tests} required tests; zero unexpected skips.`);
}
function parseArgs(args) {
  const options = {};
  for (let i = 0; i < args.length; i++) {
    const key = args[i];
    if (key === '--skip-build') options.skipBuild = true;
    else if (key === '--build-only') options.buildOnly = true;
    else if (key === '--verify-only') options.verifyOnly = true;
    else {
      const names = { '--powershell': 'powershell', '--binary': 'binary', '--provenance': 'provenance' };
      check(names[key] && i + 1 < args.length && !options[names[key]], 'argument_invalid');
      options[names[key]] = args[++i];
    }
  }
  return options;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  Promise.resolve().then(() => runGate(parseArgs(process.argv.slice(2)))).catch(error => {
    console.error(error instanceof GateError ? error.message : 'input-pipeline: internal_failure');
    process.exitCode = 1;
  });
}
