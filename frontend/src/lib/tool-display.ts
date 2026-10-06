import { t } from './i18n.svelte'

export type ToolCategory = 'terminal' | 'read' | 'edit' | 'search' | 'list' | 'generic'

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
      actionLabel = finished ? t('tools.runDone') : t('tools.runDoing')
      break
    case 'read':
      actionLabel = finished ? t('tools.readDone') : t('tools.readDoing')
      break
    case 'edit':
      actionLabel = finished ? t('tools.editDone') : t('tools.editDoing')
      break
    case 'search':
      actionLabel = finished ? t('tools.searchDone') : t('tools.searchDoing')
      break
    case 'list':
      actionLabel = finished ? t('tools.listDone') : t('tools.listDoing')
      break
    default:
      actionLabel = finished ? t('tools.genericDone') : t('tools.genericDoing')
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
  if (!tools || tools.length === 0) return t('tools.count0')
  const countStr = tools.length === 1 ? t('tools.count1') : t('tools.countN', { n: tools.length })
  const labels: string[] = []
  for (const tool of tools) {
    const cat = parseToolDisplay(tool.toolName).category
    let name = ''
    switch (cat) {
      case 'read': name = t('tools.cat.read'); break
      case 'edit': name = t('tools.cat.edit'); break
      case 'terminal': name = t('tools.cat.terminal'); break
      case 'search': name = t('tools.cat.search'); break
      case 'list': name = t('tools.cat.list'); break
      default: name = tool.toolName || t('tools.cat.fallback'); break
    }
    if (!labels.includes(name)) labels.push(name)
  }
  if (labels.length > 0) {
    return `${countStr} (${labels.slice(0, 3).join(', ')})`
  }
  return countStr
}
