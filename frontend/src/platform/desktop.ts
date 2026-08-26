type DesktopApp = {
  BackendReady?: () => Promise<boolean>
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: DesktopApp
      }
    }
  }
}

export const desktop = {
  async backendReady(): Promise<boolean> {
    const method = window.go?.main?.App?.BackendReady
    if (!method) return false
    return method()
  },
}
