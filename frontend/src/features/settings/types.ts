export type Theme = 'system' | 'light' | 'dark'

export type SettingsPayload = {
  theme: Theme
  provider: string
  credential_provider?: string
  provider_only?: boolean
  model: string
  thinking: string
  api_key: string
  custom_url: string
}
