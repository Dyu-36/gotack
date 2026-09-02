<script lang="ts">
  type Props = {
    label?: string
  }

  let { label = 'Đang làm việc…' }: Props = $props()
</script>

<div class="agent-working" role="status" aria-label={label}>
  <svg class="agent-working-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="3.25" />
    <path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.64 5.64l2.12 2.12M16.24 16.24l2.12 2.12M18.36 5.64l-2.12 2.12M7.76 16.24l-2.12 2.12" />
  </svg>
  <span class="agent-working-text">{label}</span>
</div>
