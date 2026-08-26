declare global {
  interface Window {
    go?: {
      main?: {
        App?: Record<string, (...args: unknown[]) => Promise<unknown>>
      }
    }
  }
}

export function hasDesktopBridge(): boolean {
  return Boolean(window.go?.main?.App)
}

export async function callDesktop<T>(method: string, ...args: unknown[]): Promise<T> {
  const fn = window.go?.main?.App?.[method]
  if (!fn) throw new Error(`Wails method not available: App.${method}`)
  return fn(...args) as Promise<T>
}
