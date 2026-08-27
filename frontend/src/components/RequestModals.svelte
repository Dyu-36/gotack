<script lang="ts">
  import type { PermissionRequestEvent, QuestionRequestEvent } from '../platform/desktop'

  type Answer = { request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }
  type Props = {
    permission: PermissionRequestEvent | null
    question: QuestionRequestEvent | null
    onPermission: (decision: 'allow' | 'allow_session' | 'deny') => void
    onQuestion: (answers: Answer[]) => void
  }

  let { permission, question, onPermission, onQuestion }: Props = $props()
  let values = $state<Record<string, { selected: string[]; text: string; yes: boolean | null }>>({})

  $effect(() => {
    const id = question?.id
    if (!id || !question) {
      values = {}
      return
    }
    const next: typeof values = {}
    for (const item of question.questions) next[item.id] = { selected: [], text: '', yes: null }
    values = next
  })

  function selectOne(questionID: string, choiceID: string) {
    values[questionID].selected = [choiceID]
  }

  function toggleMany(questionID: string, choiceID: string) {
    const selected = values[questionID].selected
    values[questionID].selected = selected.includes(choiceID)
      ? selected.filter((id) => id !== choiceID)
      : [...selected, choiceID]
  }

  function submitQuestions() {
    if (!question) return
    const answers: Answer[] = question.questions.map((item) => {
      const value = values[item.id] ?? { selected: [], text: '', yes: null }
      if (item.type === 'yes_no') return { request_id: item.id, yes: value.yes }
      if (item.type === 'free_text') return { request_id: item.id, fill_in_text: value.text.trim() }
      return { request_id: item.id, selected_ids: value.selected }
    })
    onQuestion(answers)
  }

  let canSubmit = $derived.by(() => {
    if (!question) return false
    return question.questions.every((item) => {
      const value = values[item.id]
      if (!value) return false
      if (item.type === 'yes_no') return value.yes !== null
      if (item.type === 'free_text') return value.text.trim().length > 0
      if (item.type === 'single_choice') return value.selected.length === 1
      if (item.type === 'multi_choice') return value.selected.length > 0
      return false
    })
  })
</script>

{#if permission}
  <div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4">
    <section class="w-full max-w-lg rounded-xl border border-mm-border bg-mm-bg shadow-xl" role="dialog" aria-modal="true" aria-label="Permission request">
      <header class="px-5 py-4 border-b border-mm-border">
        <div class="text-xs font-semibold uppercase tracking-wider text-amber-500">Crush permission</div>
        <h2 class="mt-1 text-base font-semibold text-mm-text">{permission.tool_name || 'Tool request'}</h2>
      </header>
      <div class="p-5 space-y-3">
        {#if permission.description}<p class="text-sm text-mm-text leading-relaxed">{permission.description}</p>{/if}
        {#if permission.path}<div class="rounded-md bg-mm-panel px-3 py-2 text-xs font-mono text-mm-secondary break-all">{permission.path}</div>{/if}
        {#if permission.params}
          <details class="text-xs text-mm-secondary">
            <summary class="cursor-pointer select-none">Tool parameters</summary>
            <pre class="mt-2 max-h-40 overflow-auto rounded-md bg-mm-panel p-2 text-2xs">{JSON.stringify(permission.params, null, 2)}</pre>
          </details>
        {/if}
      </div>
      <footer class="px-5 py-3 border-t border-mm-border flex justify-end gap-2">
        <button type="button" class="btn-notion px-3 py-1.5 text-xs text-red-500" onclick={() => onPermission('deny')}>Từ chối</button>
        <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={() => onPermission('allow_session')}>Cho phép trong session</button>
        <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium" onclick={() => onPermission('allow')}>Cho phép một lần</button>
      </footer>
    </section>
  </div>
{/if}

{#if question}
  <div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4">
    <section class="w-full max-w-2xl max-h-[88vh] rounded-xl border border-mm-border bg-mm-bg shadow-xl flex flex-col" role="dialog" aria-modal="true" aria-label="Crush questions">
      <header class="px-5 py-4 border-b border-mm-border shrink-0">
        <div class="text-xs font-semibold uppercase tracking-wider text-mm-accent">Crush question</div>
        <h2 class="mt-1 text-base font-semibold text-mm-text">{question.confirm_title || (question.questions.length > 1 ? 'Cần thêm thông tin' : question.questions[0]?.label || 'Câu hỏi')}</h2>
        {#if question.confirm_description}<p class="mt-1 text-xs text-mm-secondary">{question.confirm_description}</p>{/if}
      </header>

      <div class="p-5 overflow-y-auto space-y-5">
        {#each question.questions as item, index (item.id)}
          <fieldset class="space-y-2.5">
            <legend class="text-sm font-medium text-mm-text"><span class="text-mm-tertiary mr-1">{index + 1}.</span>{item.question}</legend>
            {#if item.description}<p class="text-xs text-mm-secondary">{item.description}</p>{/if}

            {#if item.type === 'yes_no'}
              <div class="flex gap-2">
                <button type="button" class:active={values[item.id]?.yes === true} class="choice-btn" onclick={() => (values[item.id].yes = true)}>Có</button>
                <button type="button" class:active={values[item.id]?.yes === false} class="choice-btn" onclick={() => (values[item.id].yes = false)}>Không</button>
              </div>
            {:else if item.type === 'free_text'}
              <textarea class="w-full min-h-24 rounded-md border border-mm-border bg-mm-panel px-3 py-2 text-sm text-mm-text outline-none focus:border-mm-accent" bind:value={values[item.id].text} placeholder="Nhập câu trả lời…"></textarea>
            {:else if item.type === 'single_choice'}
              <div class="grid gap-2">
                {#each item.choices ?? [] as choice (choice.id)}
                  <button type="button" class:active={values[item.id]?.selected.includes(choice.id)} class="choice-btn text-left" onclick={() => selectOne(item.id, choice.id)}>
                    <span class="font-medium">{choice.label}</span>{#if choice.description}<span class="block mt-0.5 text-2xs text-mm-tertiary">{choice.description}</span>{/if}
                  </button>
                {/each}
              </div>
            {:else if item.type === 'multi_choice'}
              <div class="grid gap-2">
                {#each item.choices ?? [] as choice (choice.id)}
                  <label class="choice-btn flex items-start gap-2 cursor-pointer" class:active={values[item.id]?.selected.includes(choice.id)}>
                    <input type="checkbox" checked={values[item.id]?.selected.includes(choice.id)} onchange={() => toggleMany(item.id, choice.id)} />
                    <span><span class="font-medium">{choice.label}</span>{#if choice.description}<span class="block mt-0.5 text-2xs text-mm-tertiary">{choice.description}</span>{/if}</span>
                  </label>
                {/each}
              </div>
            {:else}
              <div class="text-xs text-red-500">Unsupported question type: {item.type}</div>
            {/if}
          </fieldset>
        {/each}
      </div>

      <footer class="px-5 py-3 border-t border-mm-border flex justify-end shrink-0">
        <button type="button" disabled={!canSubmit} class="px-4 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" onclick={submitQuestions}>Gửi câu trả lời</button>
      </footer>
    </section>
  </div>
{/if}

<style>
  .choice-btn { border: 1px solid var(--mm-border); border-radius: 7px; padding: 8px 10px; font-size: 12px; color: var(--mm-text); background: var(--mm-panel); transition: 120ms ease; }
  .choice-btn:hover { background: var(--mm-hover); }
  .choice-btn.active { border-color: var(--mm-accent); box-shadow: 0 0 0 1px var(--mm-accent); }
</style>
