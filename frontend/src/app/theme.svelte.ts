export type Theme = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'gotack.theme'
const SYSTEM_DARK_QUERY = '(prefers-color-scheme: dark)'

function isTheme(value: string | null): value is Theme {
  return value === 'system' || value === 'light' || value === 'dark'
}

export function createThemeState() {
  let value = $state<Theme>('system')
  let isDarkState = $state(false)
  let mediaQuery: MediaQueryList | undefined

  const checkIsDark = () => {
    return value === 'dark' || (value === 'system' && mediaQuery?.matches === true)
  }

  const apply = () => {
    const dark = checkIsDark()
    isDarkState = dark
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    document.documentElement.style.backgroundColor = dark ? '#191919' : '#ffffff'
  }

  const onSystemThemeChange = () => {
    if (value === 'system') apply()
  }

  const initialize = () => {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (isTheme(saved)) value = saved
    mediaQuery = window.matchMedia(SYSTEM_DARK_QUERY)
    mediaQuery.addEventListener('change', onSystemThemeChange)
    apply()
  }

  const destroy = () => {
    mediaQuery?.removeEventListener('change', onSystemThemeChange)
  }

  const set = (next: Theme) => {
    value = next
    localStorage.setItem(STORAGE_KEY, next)
    apply()
  }

  const toggle = () => {
    const next = isDarkState ? 'light' : 'dark'
    set(next)
  }

  return {
    get value() {
      return value
    },
    get isDark() {
      return isDarkState
    },
    initialize,
    destroy,
    set,
    toggle,
  }
}
