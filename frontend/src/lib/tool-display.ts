export type ToolCategory = 'terminal' | 'read' | 'edit' | 'search' | 'list' | 'mcp' | 'generic'

export type ToolDisplayInfo = {
  category: ToolCategory
  actionLabel: string
  detailLabel: string
  formattedParams: string
  isCode: boolean
}

export function extractBasename(filePath: string): string {
  if (!filePath) return ''
  const normalized = filePath.replace(/\\/g, '/')
  const parts = normalized.split('/')
  return parts.filter(Boolean).pop() ?? filePath
}

function safeParseJson(raw?: string): Record<string, unknown> | null {
  if (!raw || typeof raw !== 'string') return null
  const trimmed = raw.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return null
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {

  }
  return null
}

function tokenizeToolName(name: string): string[] {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean)
}

function classifyToolName(name: string): ToolCategory {
  const tokens = tokenizeToolName(name)
  if (tokens.length === 0) return 'generic'
  if (tokens[0] === 'mcp' || tokens.includes('schedule') || tokens.includes('subagent')) return 'mcp'
  const has = (...candidates: string[]) => candidates.some((candidate) => tokens.includes(candidate))
  if (has('command', 'cmd', 'run', 'bash', 'terminal', 'exec', 'shell')) return 'terminal'
  if (has('read', 'view', 'cat')) return 'read'
  if (has('write', 'replace', 'edit', 'patch') || (tokens.includes('create') && tokens.includes('file'))) return 'edit'
  if (has('grep', 'find', 'search', 'query')) return 'search'
  if (has('list', 'ls', 'dir', 'glob', 'tree')) return 'list'
  return 'generic'
}

export function parseToolDisplay(name?: string, rawInput?: string, finished = false): ToolDisplayInfo {
  const category = classifyToolName(name ?? '')
  const parsed = safeParseJson(rawInput)

  let detail = ''
  const isCode = category === 'terminal'

  if (category === 'terminal') {
    if (parsed) {
      detail = String(parsed.CommandLine ?? parsed.command ?? parsed.cmd ?? parsed.CommandLineString ?? '')
    }
  } else if (category === 'read') {
    if (parsed) {
      const fullPath = String(parsed.AbsolutePath ?? parsed.TargetFile ?? parsed.path ?? parsed.file_path ?? parsed.Url ?? '')
      detail = extractBasename(fullPath)
    }
  } else if (category === 'edit') {
    if (parsed) {
      const fullPath = String(parsed.TargetFile ?? parsed.AbsolutePath ?? parsed.path ?? parsed.file_path ?? '')
      detail = extractBasename(fullPath)
    }
  } else if (category === 'search') {
    if (parsed) {
      const q = String(parsed.Query ?? parsed.query ?? parsed.Pattern ?? parsed.pattern ?? '')
      detail = q ? `"${q}"` : ''
    }
  } else if (category === 'list') {
    if (parsed) {
      const p = String(parsed.DirectoryPath ?? parsed.SearchDirectory ?? parsed.path ?? '')
      detail = extractBasename(p)
    }
  }

  if (!detail && rawInput) {
    const trimmed = rawInput.trim()
    if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
      detail = trimmed.length > 60 ? `${trimmed.slice(0, 60)}…` : trimmed
    }
  }

  let actionLabel = ''
  switch (category) {
    case 'terminal':
      actionLabel = finished ? 'Đã chạy lệnh' : 'Đang chạy lệnh'
      break
    case 'read':
      actionLabel = finished ? 'Đã đọc tệp' : 'Đang đọc tệp'
      break
    case 'edit':
      actionLabel = finished ? 'Đã cập nhật tệp' : 'Đang cập nhật tệp'
      break
    case 'search':
      actionLabel = finished ? 'Đã tìm kiếm' : 'Đang tìm kiếm'
      break
    case 'list':
      actionLabel = finished ? 'Đã duyệt thư mục' : 'Đang duyệt thư mục'
      break
    case 'mcp':
      actionLabel = finished ? 'Đã gọi MCP' : 'Đang gọi MCP'
      break
    default:
      actionLabel = finished ? 'Hoàn thành công cụ' : 'Đang thực thi'
      break
  }

  let formattedParams = rawInput?.trim() ?? ''
  if (parsed) {
    try {
      formattedParams = JSON.stringify(parsed, null, 2)
    } catch {}
  }

  return {
    category,
    actionLabel,
    detailLabel: detail || (name ?? 'tool'),
    formattedParams,
    isCode,
  }
}

export function formatToolGroupSummary(tools: readonly { toolName?: string }[]): string {
  if (!tools || tools.length === 0) return '0 công cụ'
  const countStr = tools.length === 1 ? '1 công cụ' : `${tools.length} công cụ`
  const labels: string[] = []
  for (const t of tools) {
    const cat = parseToolDisplay(t.toolName).category
    let name = ''
    switch (cat) {
      case 'read': name = 'đọc tệp'; break
      case 'edit': name = 'sửa tệp'; break
      case 'terminal': name = 'chạy lệnh'; break
      case 'search': name = 'tìm kiếm'; break
      case 'list': name = 'duyệt thư mục'; break
      case 'mcp': name = 'mcp'; break
      default: name = t.toolName || 'công cụ'; break
    }
    if (!labels.includes(name)) labels.push(name)
  }
  if (labels.length > 0) {
    return `${countStr} (${labels.slice(0, 3).join(', ')})`
  }
  return countStr
}
