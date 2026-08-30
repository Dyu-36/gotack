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
