<script lang="ts">
  import type { RenderedMarkdownBlock } from '../lib/markdown'

  type Props = {
    block: RenderedMarkdownBlock
    streaming?: boolean
  }

  let { block, streaming = false }: Props = $props()
</script>

<div
  class="streaming-markdown-block"
  data-streaming-tail={streaming ? 'true' : undefined}
  data-streaming-block={block.type}
>
  {@html block.html}
</div>
