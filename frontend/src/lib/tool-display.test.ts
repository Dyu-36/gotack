import { describe, expect, it } from 'vitest'
import { extractBasename, parseToolDisplay, formatToolGroupSummary } from './tool-display'

describe('extractBasename', () => {
  it('extracts filename from windows path', () => {
    expect(extractBasename('D:\\gotack\\frontend\\src\\app.css')).toBe('app.css')
  })

  it('extracts filename from posix path', () => {
    expect(extractBasename('/var/log/syslog')).toBe('syslog')
  })

  it('handles single filename or empty', () => {
    expect(extractBasename('main.go')).toBe('main.go')
    expect(extractBasename('')).toBe('')
  })
})

describe('parseToolDisplay', () => {
  it('parses run_command with CommandLine', () => {
    const input = JSON.stringify({ CommandLine: 'pnpm test' })
    const res = parseToolDisplay('run_command', input, false)
    expect(res.category).toBe('terminal')
    expect(res.actionLabel).toBe('Đang chạy lệnh')
    expect(res.detailLabel).toBe('pnpm test')
    expect(res.isCode).toBe(true)

    const finished = parseToolDisplay('run_command', input, true)
    expect(finished.actionLabel).toBe('Đã chạy lệnh')
  })

  it('parses view_file with AbsolutePath', () => {
    const input = JSON.stringify({ AbsolutePath: 'D:\\gotack\\main.go' })
    const res = parseToolDisplay('view_file', input, false)
    expect(res.category).toBe('read')
    expect(res.actionLabel).toBe('Đang đọc tệp')
    expect(res.detailLabel).toBe('main.go')
  })

  it('parses write_to_file with TargetFile', () => {
    const input = JSON.stringify({ TargetFile: 'D:/gotack/frontend/src/app.css' })
    const res = parseToolDisplay('write_to_file', input, true)
    expect(res.category).toBe('edit')
    expect(res.actionLabel).toBe('Đã cập nhật tệp')
    expect(res.detailLabel).toBe('app.css')
  })

  it('parses grep_search with Query', () => {
    const input = JSON.stringify({ Query: 'toolActivity' })
    const res = parseToolDisplay('grep_search', input, false)
    expect(res.category).toBe('search')
    expect(res.actionLabel).toBe('Đang tìm kiếm')
    expect(res.detailLabel).toBe('"toolActivity"')
  })

  it('parses list_dir with DirectoryPath', () => {
    const input = JSON.stringify({ DirectoryPath: 'D:/gotack/frontend' })
    const res = parseToolDisplay('list_dir', input, true)
    expect(res.category).toBe('list')
    expect(res.actionLabel).toBe('Đã duyệt thư mục')
    expect(res.detailLabel).toBe('frontend')
  })

  it('handles malformed JSON gracefully', () => {
    const res = parseToolDisplay('run_command', '{"incomplete": ', false)
    expect(res.category).toBe('terminal')
    expect(res.detailLabel).toBe('run_command')
  })

  it('handles plain text input gracefully', () => {
    const res = parseToolDisplay('unknown_tool', 'simple string info', true)
    expect(res.category).toBe('generic')
    expect(res.actionLabel).toBe('Hoàn thành công cụ')
    expect(res.detailLabel).toBe('simple string info')
  })
})

describe('formatToolGroupSummary', () => {
  it('formats empty or single tool', () => {
    expect(formatToolGroupSummary([])).toBe('0 công cụ')
    expect(formatToolGroupSummary([{ toolName: 'run_command' }])).toBe('1 công cụ (chạy lệnh)')
  })

  it('formats multiple unique categories', () => {
    const tools = [
      { toolName: 'view_file' },
      { toolName: 'write_to_file' },
      { toolName: 'run_command' },
    ]
    expect(formatToolGroupSummary(tools)).toBe('3 công cụ (đọc tệp, sửa tệp, chạy lệnh)')
  })

  it('deduplicates repeating tool categories', () => {
    const tools = [
      { toolName: 'view_file' },
      { toolName: 'read_resource' },
    ]
    expect(formatToolGroupSummary(tools)).toBe('2 công cụ (đọc tệp)')
  })
})
