<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { desktop, type MigrationPreview, type MigrationStatus } from '../platform/desktop'

  const modeLabels: Record<string, string> = {
    legacy: 'Đang dùng TACK.md cũ',
    pending: 'Chờ chuyển đổi',
    staged: 'Đang chuyển đổi',
    'committed-layered': 'Đã phân lớp: TACK_CORE + USER',
    'rolled-back': 'Đã hoàn tác về TACK.md',
  }

  let preview = $state<MigrationPreview | null>(null)
  let loading = $state(false)
  let busy = $state(false)
  let loadError = $state('')
  let resolvedUser = $state('')
  let conflictNotice = $state('')

  const status: MigrationStatus | null = $derived(preview?.status ?? null)
  const needsResolution: boolean = $derived(preview?.requires_resolution === true)

  function errorMessage(cause: unknown): string {
    return cause instanceof Error ? cause.message : String(cause)
  }

  function isCasConflict(message: string): boolean {
    return /context migration changed since preview/i.test(message)
  }

  async function load() {
    loading = true
    loadError = ''
    try {
      const next = await desktop.contextMigrationPreview()
      preview = next
      resolvedUser = next.candidate_user ?? next.user_context ?? ''
    } catch (cause) {
      preview = null
      loadError = errorMessage(cause)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    void load()
  })

  function hasMergeMarkers(text: string): boolean {
    return text.includes('<<<<<<<') || text.includes('>>>>>>>') || text.includes('|||||||')
  }

  async function accept() {
    if (!preview || !status) return
    if (hasMergeMarkers(resolvedUser)) {
      conflictNotice = 'Nội dung USER.md còn chứa dấu xung đột (<<<<<<< … >>>>>>>). Hãy xử lý hết các khối xung đột rồi thử lại.'
      return
    }
    busy = true
    conflictNotice = ''
    try {
      const next = await desktop.acceptContextMigration({
        expected_generation: status.generation,
        expected_legacy_hash: status.legacy_hash ?? '',
        expected_user_hash: status.user_hash ?? '',
        expected_core_hash: status.core_hash ?? '',
        resolved_user: resolvedUser,
      })
      toast.success(modeLabels[next.mode] ?? 'Đã cập nhật ngữ cảnh')
      await load()
    } catch (cause) {
      const message = errorMessage(cause)
      if (isCasConflict(message)) {
        conflictNotice = 'Nội dung ngữ cảnh đã thay đổi từ lúc xem trước (tác vụ khác hoặc phiên khác). Tải lại trạng thái mới nhất rồi xác nhận lại.'
      } else {
        toast.error(message)
      }
    } finally {
      busy = false
    }
  }

  async function rollback() {
    if (!status?.backup_token) return
    if (!window.confirm('Khôi phục TACK.md cũ từ bản lưu và bỏ phân lớp TACK_CORE/USER?')) return
    busy = true
    conflictNotice = ''
    try {
      const next = await desktop.rollbackContextMigration({
        expected_generation: status.generation,
        token: status.backup_token,
      })
      toast.success(modeLabels[next.mode] ?? 'Đã hoàn tác ngữ cảnh')
      await load()
    } catch (cause) {
      const message = errorMessage(cause)
      if (isCasConflict(message)) {
        conflictNotice = 'Nội dung ngữ cảnh đã thay đổi từ lúc xem trước. Tải lại trạng thái mới nhất rồi thử lại.'
      } else {
        toast.error(message)
      }
    } finally {
      busy = false
    }
  }

  function short(hash: string | undefined): string {
    return hash ? `${hash.slice(0, 12)}…` : '—'
  }

  function formatTime(ms: number): string {
    if (!ms) return '—'
    return new Date(ms).toLocaleString()
  }

  function clip(text: string | undefined, max = 6000): string {
    if (!text) return ''
    return text.length > max ? `${text.slice(0, max)}\n… (còn nữa, hãy tải đầy đủ bằng trình soạn thảo ngoài)` : text
  }
</script>

<section class="migration-panel">
  <div class="section-title">Ngữ cảnh trợ lý</div>
  <p class="hint">
    Gotack quản lý phần cốt lõi trong <code>TACK_CORE.md</code> và giữ phần bạn tùy biến trong
    <code>USER.md</code>. File <code>TACK.md</code> cũ được chuyển sang mô hình hai lớp bằng một giao dịch
    an toàn, có bản lưu để hoàn tác.
  </p>

  {#if loading && !preview}
    <p class="hint">Đang tải trạng thái chuyển đổi...</p>
  {:else if loadError}
    <div class="notice">
      <span>Không tải được trạng thái chuyển đổi: {loadError}</span>
      <button type="button" class="btn-notion text-xs" onclick={() => void load()}>Thử lại</button>
    </div>
  {:else if status}
    <div class="status-row">
      <span class="mode-badge" data-mode={status.mode}>{modeLabels[status.mode] ?? status.mode}</span>
      <span class="meta">Phiên {status.generation} · cập nhật {formatTime(status.updated_at)}</span>
    </div>

    {#if conflictNotice}
      <div class="notice conflict" role="alert">
        <strong>Xung đột cập nhật:</strong> {conflictNotice}
        <button type="button" class="btn-notion text-xs" disabled={busy} onclick={() => void load()}>Tải lại trạng thái</button>
      </div>
    {/if}

    {#if status.stage}
      <div class="notice">
        Có một lượt chuyển đổi đang dở dang từ lần chạy trước. Gotack sẽ tự kiểm tra và hoàn tất an toàn ở
        lần xem tiếp theo; không cần bạn can thiệp.
      </div>
    {:else if needsResolution}
      <p class="hint">
        Gotack tìm thấy nội dung cũ chưa chuyển đổi. Xem trước nội dung bên dưới, chỉnh <code>USER.md</code>
        nếu cần, rồi xác nhận chuyển đổi. File cũ được lưu vào bản dự phòng trước khi bị thay thế.
      </p>

      <details class="diff-box">
        <summary>TACK.md cũ ({short(status.legacy_hash)})</summary>
        <pre>{clip(preview?.legacy) || '(trống)'}</pre>
      </details>
      {#if preview?.known_base}
        <details class="diff-box">
          <summary>Bản gốc đã biết (so sánh ba chiều)</summary>
          <pre>{clip(preview.known_base)}</pre>
        </details>
      {/if}

      {#if preview?.has_conflicts}
        <div class="notice conflict">
          <strong>Cần bạn xử lý:</strong> nội dung cũ khác với bản gốc Gotack đã biết. Khung
          <code>USER.md</code> bên dưới chứa các khối xung đột ba chiều — hãy giữ phần bạn muốn rồi xóa hết
          các dòng đánh dấu trước khi xác nhận.
        </div>
      {:else}
        <p class="hint">Không phát hiện xung đột: xác nhận để áp dụng nội dung đề xuất.</p>
      {/if}

      <label class="field-label" for="migration-user-context">USER.md sau khi chuyển đổi</label>
      <textarea id="migration-user-context" class="field font-mono" rows="10" bind:value={resolvedUser} spellcheck="false"></textarea>

      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="btn-notion text-xs" disabled={busy} onclick={() => void load()}>Tải lại</button>
        <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={busy} onclick={() => void accept()}>
          {busy ? 'Đang xử lý...' : 'Chấp nhận chuyển đổi'}
        </button>
      </div>
    {:else if status.mode === 'committed-layered'}
      <p class="hint">
        Ngữ cảnh đang phân hai lớp: <code>TACK_CORE.md</code> do Gotack cập nhật theo bản phát hành, và
        <code>USER.md</code> do bạn quản lý, không bị ghi đè khi nâng cấp.
      </p>
      {#if status.backup_token}
        <div class="notice">
          <span>Bản dự phòng TACK.md cũ: <code>{status.backup_token}</code></span>
          <div class="flex flex-wrap justify-end gap-2 pt-1">
            <button type="button" class="btn-danger text-xs" disabled={busy} onclick={() => void rollback()}>
              {busy ? 'Đang xử lý...' : 'Hoàn tác về TACK.md cũ'}
            </button>
          </div>
        </div>
      {/if}
    {:else}
      <div class="notice">Ngữ cảnh đã dùng mô hình hai lớp.</div>
    {/if}
  {/if}
</section>

<style>
  .migration-panel { display: grid; gap: 10px; }
  .section-title { font-size: 11px; font-weight: 700; color: var(--mm-tertiary); letter-spacing: .08em; text-transform: uppercase; }
  .hint { margin: 0; font-size: 11px; line-height: 1.55; color: var(--mm-tertiary); }
  .hint code { font-size: 10px; }
  .notice { padding: 9px 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 11px; line-height: 1.55; display: grid; gap: 4px; }
  .notice.conflict { border-color: rgb(245 158 11 / 45%); background: rgb(245 158 11 / 8%); }
  .status-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
  .mode-badge { padding: 3px 9px; border-radius: 999px; border: 1px solid var(--mm-border); background: var(--mm-panel); font-size: 11px; font-weight: 600; color: var(--mm-text); }
  .mode-badge[data-mode='committed-layered'] { border-color: rgb(16 185 129 / 45%); color: #10b981; }
  .mode-badge[data-mode='pending'], .mode-badge[data-mode='staged'] { border-color: rgb(245 158 11 / 45%); color: #d97706; }
  .meta { font-size: 10px; color: var(--mm-tertiary); }
  .field-label { font-size: 12px; font-weight: 600; color: var(--mm-secondary); }
  .field { width: 100%; padding: 7px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font-size: 12px; outline: none; resize: vertical; }
  .field:focus { border-color: var(--mm-accent); }
  .diff-box { border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); }
  .diff-box summary { padding: 7px 9px; font-size: 11px; font-weight: 600; color: var(--mm-secondary); cursor: pointer; user-select: none; }
  .diff-box pre { margin: 0; max-height: 220px; overflow: auto; padding: 8px 9px; border-top: 1px solid var(--mm-border); font-size: 10.5px; line-height: 1.5; color: var(--mm-secondary); white-space: pre-wrap; word-break: break-word; }
</style>
