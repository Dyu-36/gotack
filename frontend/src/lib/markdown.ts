import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({ breaks: true, gfm: true })

const ALLOWED_URI_REGEXP =
  /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp|file):|[a-z]:[\\/]|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i

function sanitize(html: string): string {
  return DOMPurify.sanitize(html, { ALLOWED_URI_REGEXP })
}

function escapeFallback(content: string): string {
  return content
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\n/g, '<br/>')
}

/**
 * Parses markdown content into sanitized HTML.
 * Parses the entire content string to maintain structural integrity
 * for lists, tables, and code blocks.
 */
export function renderMarkdown(content: string): string {
  if (!content) return ''
  try {
    return sanitize(marked.parse(content, { async: false }) as string)
  } catch {
    return sanitize(escapeFallback(content))
  }
}

type MarkedTokens = ReturnType<typeof marked.lexer>
type MarkedToken = MarkedTokens[number]

export type RenderedMarkdownBlock = {
  id: string
  type: string
  raw: string
  linksKey: string
  html: string
}

let fallbackBlockSequence = 0

function renderToken(token: MarkedToken, links: MarkedTokens['links']): string {
  try {
    const singleTokenList = [token] as unknown as MarkedTokens
    singleTokenList.links = links
    return sanitize(marked.parser(singleTokenList))
  } catch {
    return sanitize(escapeFallback(token.raw ?? ''))
  }
}

/**
 * Renders top-level Markdown tokens independently so settled blocks keep their
 * DOM identity while only the live tail is reparsed. The full lexer still sees
 * the whole document, preserving reference-link definitions and Markdown block
 * boundaries; cached blocks skip both HTML generation and sanitization.
 */
export function renderMarkdownBlocks(
  content: string,
  previous: readonly RenderedMarkdownBlock[] = [],
  createId: () => string = () => `markdown-block-${++fallbackBlockSequence}`,
): RenderedMarkdownBlock[] {
  if (!content) return []

  let tokens: MarkedTokens
  try {
    tokens = marked.lexer(content)
  } catch {
    const prior = previous[0]
    const raw = content
    if (prior?.type === 'fallback' && prior.raw === raw) return [prior]
    return [{
      id: prior?.id ?? createId(),
      type: 'fallback',
      raw,
      linksKey: '',
      html: sanitize(escapeFallback(raw)),
    }]
  }

  const links = tokens.links ?? {}
  const linksKey = JSON.stringify(links)
  const visibleTokens = tokens.filter((token) =>
    token.type !== 'space' &&
    token.type !== 'def' &&
    typeof token.raw === 'string' &&
    token.raw.length > 0,
  )

  return visibleTokens.map((token, index) => {
    const prior = previous[index]
    if (
      prior &&
      prior.type === token.type &&
      prior.raw === token.raw &&
      prior.linksKey === linksKey
    ) {
      return prior
    }

    return {
      id: prior?.id ?? createId(),
      type: token.type,
      raw: token.raw,
      linksKey,
      html: renderToken(token, links),
    }
  })
}

export function openChatLink(href: string): void {
  if (!href || href.startsWith('#')) return

  try {
    const rt = (window as unknown as { runtime?: { BrowserOpenURL?: (url: string) => void } }).runtime
    if (typeof rt?.BrowserOpenURL === 'function') {
      rt.BrowserOpenURL(href)
      return
    }
  } catch (err) {
    console.warn('BrowserOpenURL failed:', err)
  }

  window.open(href, '_blank', 'noopener,noreferrer')
}

export function chatLinks(node: HTMLElement) {
  function onClick(event: MouseEvent) {
    const target = event.target as HTMLElement | null
    const anchor = target?.closest?.('a') as HTMLAnchorElement | null
    if (!anchor) return

    const href = anchor.getAttribute('href')
    if (!href || href.startsWith('#')) return

    event.preventDefault()
    openChatLink(href)
  }

  node.addEventListener('click', onClick)
  return {
    destroy() {
      node.removeEventListener('click', onClick)
    },
  }
}
