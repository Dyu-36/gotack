import type { ChatAttachment } from './types.svelte'

export const MAX_ATTACHMENT_SIZE = 5 * 1024 * 1024

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

export async function fileToAttachment(file: File): Promise<ChatAttachment> {
  if (file.size > MAX_ATTACHMENT_SIZE) {
    throw new Error(`Tệp “${file.name}” vượt quá giới hạn 5 MB`)
  }

  const content = await readFileAsBase64(file)
  return {
    id: `attachment:${Date.now().toString(36)}:${++attachmentSeq}`,
    fileName: file.name || `clipboard-${Date.now()}.png`,
    mimeType: file.type || 'application/octet-stream',
    size: file.size,
    content,
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
