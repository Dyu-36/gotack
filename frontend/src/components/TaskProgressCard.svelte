<script lang="ts">
  import type { Message } from '../features/conversations/types.svelte'

  type Props = { message: Message }
  let { message }: Props = $props()

  let label = $derived.by(() => {
    const elapsed = message.taskElapsedSeconds ?? 0
    const limit = message.taskLimitSeconds ?? 90
    switch (message.taskState) {
      case 'searching': return `Đang xếp thời khóa biểu · ${elapsed}/${limit} giây`
      case 'optimizing': return 'Đã tìm thấy lịch hợp lệ · đang tối ưu'
      case 'optimal': return 'Đã xếp lịch tối ưu'
      case 'feasible': {
        const count = message.taskSoftViolationCount ?? 0
        return count > 0
          ? `Đã xếp lịch hợp lệ · còn ${count} yêu cầu ưu tiên chưa đạt`
          : 'Đã xếp lịch hợp lệ'
      }
      case 'infeasible': return 'Không thể xếp với các ràng buộc bắt buộc hiện tại'
      case 'timed_out': return 'Chưa có kết quả trong thời gian cho phép'
      case 'failed': return 'Tác vụ xếp thời khóa biểu gặp lỗi vận hành'
      default: return 'Đang xếp thời khóa biểu'
    }
  })

  let running = $derived(message.taskState === 'searching' || message.taskState === 'optimizing')
</script>

<div class="task-progress-card mb-3 mr-8 sm:mr-20" class:is-running={running} role="status" aria-live="polite">
  <div class="flex items-center gap-2.5 px-3 py-2.5">
    <div class="w-2 h-2 rounded-full bg-mm-accent shrink-0" class:pulse={running}></div>
    <div class="min-w-0 flex-1">
      <div class="text-sm font-medium text-mm-text">{label}</div>
      {#if message.taskPenalty !== undefined && (message.taskState === 'feasible' || message.taskState === 'optimal')}
        <div class="text-xs text-mm-secondary mt-0.5">Tổng penalty: {message.taskPenalty}</div>
      {/if}
    </div>
  </div>
</div>

<style>
  .task-progress-card {
    margin-left: 36px;
    border: 1px solid var(--mm-border);
    border-radius: 9px;
    background: var(--mm-panel);
  }
  .task-progress-card.is-running {
    border-color: color-mix(in srgb, var(--mm-accent) 40%, var(--mm-border));
  }
  .pulse { animation: task-pulse 1.4s ease-in-out infinite; }
  @keyframes task-pulse { 50% { opacity: 0.35; transform: scale(0.8); } }
  @media (prefers-reduced-motion: reduce) { .pulse { animation: none; } }
</style>
