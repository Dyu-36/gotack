import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const repoRoot = path.resolve(import.meta.dirname, '..')

function fail(message) {
  console.error(`repository invariant failed: ${message}`)
  process.exitCode = 1
}

function trackedFiles(...patterns) {
  const output = execFileSync('git', ['ls-files', '--', ...patterns], {
    cwd: repoRoot,
    encoding: 'utf8',
  })
  return output.split(/\r?\n/).filter(Boolean)
}

function checkImplementationFileSize() {
  const files = trackedFiles(
    '*.go',
    '*.ts',
    '*.tsx',
    '*.js',
    '*.jsx',
    '*.svelte',
    '*.mjs',
    '*.cjs',
    '*.py',
    '*.ps1',
  )
  for (const file of files) {
    const absolutePath = path.join(repoRoot, file)
    if (!existsSync(absolutePath)) continue
    const text = readFileSync(absolutePath, 'utf8')
    const lineCount = text.length === 0 ? 0 : text.replace(/\r\n/g, '\n').split('\n').length - (text.endsWith('\n') ? 1 : 0)
    if (lineCount >= 1000) {
      fail(`${file} has ${lineCount} lines; AGENTS.md hard rule 6 requires implementation files to stay under 1000 lines. Split it by responsibility.`)
    }
  }
}

function extract(file, pattern, description) {
  const text = readFileSync(path.join(repoRoot, file), 'utf8')
  const match = text.match(pattern)
  if (!match) {
    fail(`${file} does not expose the ${description} expected by the Crush pin consistency check. Update this checker with the owning format.`)
    return null
  }
  return match[1]
}

function checkCrushPin() {
  // Single tracked owner: the repository-root `.crush-pin` file. The
  // workflows and scripts/update-crush.ps1 read it at run time.
  const pinText = readFileSync(path.join(repoRoot, '.crush-pin'), 'utf8').trim()
  if (!/^[0-9a-f]{40}$/.test(pinText)) {
    fail(`.crush-pin must contain exactly one 40-character lowercase SHA, got "${pinText}".`)
  }

  // Drift guard: the previous owners must not grow a hardcoded SHA again, or
  // the pin silently splits into hand-synced copies.
  const sha = /\b[0-9a-f]{40}\b/
  const retired = [
    '.github/workflows/ci.yml',
    '.github/workflows/release.yml',
    'scripts/update-crush.ps1',
  ]
  for (const file of retired) {
    const text = readFileSync(path.join(repoRoot, file), 'utf8')
    const match = text.match(sha)
    if (match) {
      fail(`${file} contains a hardcoded Crush SHA (${match[0]}). The pin has one owner: .crush-pin.`)
    }
  }

  // The documents reference the owning file instead of a SHA.
  extract('third_party/README.md', /\.crush-pin/, 'reference to the .crush-pin owner')
  extract('README.md', /\| Crush pin \| \*\*`\.crush-pin`\*\*/, 'README Crush pin row pointing at .crush-pin')
}

checkImplementationFileSize()
checkCrushPin()

if (process.exitCode) process.exit(process.exitCode)
console.log('Repository invariants passed: implementation file size and Crush pin consistency.')
