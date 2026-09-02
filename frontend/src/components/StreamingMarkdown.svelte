<script lang="ts">
  import { onDestroy } from 'svelte'
  import MarkdownBlock from './MarkdownBlock.svelte'
  import { renderMarkdownBlocks, type RenderedMarkdownBlock } from '../lib/markdown'
  import { nextStreamingText } from '../lib/streaming-text'

  type Props = {
    content: string
    isStreaming?: boolean
  }

  let { content, isStreaming = false }: Props = $props()

  let displayedContent = $state('')
  let blocks = $state<RenderedMarkdownBlock[]>([])
  let renderedBlocks: RenderedMarkdownBlock[] = []
  let targetContent = ''
  let streamActive = false
  let reducedMotion = false
  let frame: number | null = null
  let blockSequence = 0

  const createBlockId = () => `stream-block-${++blockSequence}`

  function cancelFrame() {
    if (frame === null) return
    cancelAnimationFrame(frame)
    frame = null
  }

  function scheduleFrame() {
    if (frame !== null) return
    frame = requestAnimationFrame(advance)
  }

  function advance() {
    frame = null

    if (!streamActive || reducedMotion) {
      if (displayedContent !== targetContent) displayedContent = targetContent
      return
    }

    const next = nextStreamingText(displayedContent, targetContent)
    if (next !== displayedContent) displayedContent = next
    if (next !== targetContent) scheduleFrame()
  }

  $effect(() => {
    targetContent = content ?? ''
    streamActive = isStreaming
    reducedMotion =
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches

    if (!streamActive || reducedMotion || !targetContent.startsWith(displayedContent)) {
      cancelFrame()
      displayedContent = targetContent
      return
    }

    if (displayedContent !== targetContent) scheduleFrame()
  })

  $effect(() => {
    renderedBlocks = renderMarkdownBlocks(displayedContent, renderedBlocks, createBlockId)
    blocks = renderedBlocks
  })

  onDestroy(cancelFrame)
</script>

{#each blocks as block, index (block.id)}
  <MarkdownBlock {block} streaming={isStreaming && index === blocks.length - 1} />
{/each}
