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
      fail(`${file} has ${lineCount} lines; implementation files must stay under 1000 lines. Split it by responsibility.`)
    }
  }
}

checkImplementationFileSize()

if (process.exitCode) process.exit(process.exitCode)
console.log('Repository invariants passed: implementation file size.')
