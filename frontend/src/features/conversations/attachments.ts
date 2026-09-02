import type { ChatAttachment } from './types.svelte'
import type { PromptFilePick } from '../../platform/desktop'

const FALLBACK_MAX_ATTACHMENT_SIZE = 5 * 1024 * 1024
let maxAttachmentSize = FALLBACK_MAX_ATTACHMENT_SIZE

export function setAttachmentLimit(bytes: number): void {
  if (Number.isFinite(bytes) && bytes > 0) maxAttachmentSize = Math.floor(bytes)
}

export function attachmentLimit(): number {
  return maxAttachmentSize
}

let attachmentSeq = 0

const previewableImageTypes = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp'])

export function isPreviewableImage(mimeType: string): boolean {
  return previewableImageTypes.has(mimeType.toLowerCase())
}

export function attachmentDataURL(attachment: ChatAttachment): string {
  return `data:${attachment.mimeType};base64,${attachment.content}`
}

export function formatAttachmentSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${Math.ceil(size / 1024)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

const extensionMimeTypes: Record<string, string> = {
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  xlsm: 'application/vnd.ms-excel.sheet.macroEnabled.12',
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  ppt: 'application/vnd.ms-powerpoint',
  pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  csv: 'text/csv',
  txt: 'text/plain',
  md: 'text/markdown',
  json: 'application/json',
  pdf: 'application/pdf',
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
}

export function mimeFromName(fileName: string): string {
  const dot = fileName.lastIndexOf('.')
  if (dot < 0) return 'application/octet-stream'
  return extensionMimeTypes[fileName.slice(dot + 1).toLowerCase()] ?? 'application/octet-stream'
}

export async function fileToAttachment(file: File): Promise<ChatAttachment> {
  if (file.size > maxAttachmentSize) {
    throw new Error(`Tệp “${file.name}” vượt quá hạn mức ${formatAttachmentSize(maxAttachmentSize)}`)
  }

  const content = await readFileAsBase64(file)
  const fileName = file.name || `clipboard-${Date.now()}.png`
  return {
    id: `attachment:${Date.now().toString(36)}:${++attachmentSeq}`,
    fileName,
    mimeType: file.type || mimeFromName(fileName),
    size: file.size,
    content,
  }
}

export function pathToAttachment(pick: PromptFilePick): ChatAttachment {
  const fileName = pick.file_name || pick.path
  return {
    id: `attachment:${Date.now().toString(36)}:${++attachmentSeq}`,
    fileName,
    mimeType: pick.mime_type || mimeFromName(fileName),
    size: pick.size,
    content: '',
    path: pick.path,
  }
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error ?? new Error(`Không thể đọc tệp “${file.name}”`))
    reader.onload = () => {
      if (typeof reader.result !== 'string') {
        reject(new Error(`Không thể đọc tệp “${file.name}”`))
        return
      }
      const comma = reader.result.indexOf(',')
      resolve(comma >= 0 ? reader.result.slice(comma + 1) : reader.result)
    }
    reader.readAsDataURL(file)
  })
}
