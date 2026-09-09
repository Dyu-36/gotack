export const NEW_CONVERSATION_TITLE = 'Hội thoại mới'

const defaultTitles = new Set(['', 'new session', 'new conversation', 'new chat', 'untitled session', 'hội thoại mới'])

export function isDefaultTitle(title: string): boolean {
  return defaultTitles.has(title.trim().toLowerCase())
}

export function conversationTitle(title: string, text = '', fileNames: readonly string[] = []): string {
  if (!isDefaultTitle(title)) return title.trim()
  const clean = (text.trim() || fileNames.join(', ')).replace(/[\p{Cc}\s]+/gu, ' ').trim()
  const chars = Array.from(clean)
  return chars.length > 80 ? `${chars.slice(0, 79).join('').trimEnd()}…` : clean || NEW_CONVERSATION_TITLE
}
