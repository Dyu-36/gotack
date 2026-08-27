<script lang="ts">
  import { onMount } from 'svelte'
  import { desktop, events, on, type TerminalDataEvent, type TerminalExitEvent } from '../platform/desktop'

  type Props = { cwd: string; onClose: () => void }
  let { cwd, onClose }: Props = $props()

  let host = $state<HTMLDivElement>()
  let terminalID = $state('')
  let error = $state('')

  onMount(() => {
    let disposed = false
    let cleanup = () => {}

    void (async () => {
      try {
        // Keep xterm out of the initial JS bundle. This component itself is
        // rendered only after the user opens the terminal panel.
        const [{ Terminal }, { FitAddon }] = await Promise.all([
          import('@xterm/xterm'),
          import('@xterm/addon-fit'),
          import('@xterm/xterm/css/xterm.css'),
        ])
        if (disposed || !host) return

        const terminal = new Terminal({
          cursorBlink: true,
          convertEol: true,
          scrollback: 3000,
          fontSize: 12,
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        })
        const fit = new FitAddon()
        terminal.loadAddon(fit)
        terminal.open(host)
        fit.fit()

        terminalID = await desktop.openTerminal(cwd)
        if (disposed) {
          await desktop.closeTerminal(terminalID).catch(() => {})
          return
        }

        const dataSub = on<TerminalDataEvent>(events.terminalData, (event) => {
          if (event.id === terminalID) terminal.write(event.data)
        })
        const exitSub = on<TerminalExitEvent>(events.terminalExit, (event) => {
          if (event.id !== terminalID) return
          terminal.writeln(`\r\n[process exited ${event.code ?? ''}]`)
          terminalID = ''
        })
        const inputSub = terminal.onData((data) => {
          if (terminalID) void desktop.writeTerminal(terminalID, data).catch((cause) => {
            error = cause instanceof Error ? cause.message : String(cause)
          })
        })

        const resize = new ResizeObserver(() => {
          if (!terminalID) return
          fit.fit()
          const { cols, rows } = terminal
          if (cols > 0 && rows > 0) void desktop.resizeTerminal(terminalID, cols, rows).catch(() => {})
        })
        resize.observe(host)
        queueMicrotask(() => {
          fit.fit()
          if (terminalID) void desktop.resizeTerminal(terminalID, terminal.cols, terminal.rows).catch(() => {})
          terminal.focus()
        })

        cleanup = () => {
          dataSub()
          exitSub()
          inputSub.dispose()
          resize.disconnect()
          terminal.dispose()
          if (terminalID) void desktop.closeTerminal(terminalID).catch(() => {})
          terminalID = ''
        }
      } catch (cause) {
        error = cause instanceof Error ? cause.message : String(cause)
      }
    })()

    return () => {
      disposed = true
      cleanup()
    }
  })

  function closePanel() {
    if (terminalID) void desktop.closeTerminal(terminalID).catch(() => {})
    terminalID = ''
    onClose()
  }
</script>

<aside class="h-full w-[min(48vw,720px)] min-w-96 border-l border-mm-border bg-[#111] flex flex-col" aria-label="Terminal">
  <header class="h-10 px-3 flex items-center justify-between border-b border-white/10 text-white shrink-0">
    <div class="min-w-0">
      <span class="text-xs font-semibold">Terminal</span>
      <span class="ml-2 text-2xs text-white/45 font-mono truncate">{cwd}</span>
    </div>
    <button type="button" class="px-2 py-1 rounded text-xs text-white/70 hover:bg-white/10 hover:text-white" onclick={closePanel}>Đóng</button>
  </header>
  {#if error}<div class="px-3 py-2 text-xs text-red-300 bg-red-950/40">{error}</div>{/if}
  <div bind:this={host} class="flex-1 min-h-0 p-1 overflow-hidden"></div>
</aside>
