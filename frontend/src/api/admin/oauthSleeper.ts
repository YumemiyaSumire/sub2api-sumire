import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type OAuthSleeperPlatform = 'openai' | 'anthropic'

export interface OAuthSleeperSettings {
  enabled: boolean
  threshold_percent: number
  scan_interval_seconds: number
  max_sleep_per_scan: number
  include_openai: boolean
  include_anthropic: boolean
  group_ids: number[]
}

export interface OAuthSleeperSleepingAccount {
  account_id: number
  account_name: string
  platform: OAuthSleeperPlatform | string
  rate_limit_reset_at: string
  remaining_seconds: number
}

export interface OAuthSleeperStatus extends OAuthSleeperSettings {
  last_scan_at?: string
  last_scanned: number
  last_triggered: number
  last_error?: string
  sleeping_accounts: OAuthSleeperSleepingAccount[]
}

export interface OAuthSleeperEvent {
  id: number
  account_id: number
  account_name: string
  platform: OAuthSleeperPlatform | string
  window: string
  utilization_percent: number
  threshold_percent: number
  reset_at: string
  previous_rate_limit_reset_at?: string | null
  created_at: string
}

export interface OAuthSleeperScanResult {
  scanned: number
  triggered: number
  events: OAuthSleeperEvent[]
}

export interface OAuthSleeperEventsParams {
  page?: number
  page_size?: number
}

export async function getStatus(): Promise<OAuthSleeperStatus> {
  const { data } = await apiClient.get<OAuthSleeperStatus>('/admin/oauth-sleeper/status')
  return data
}

export async function getSettings(): Promise<OAuthSleeperSettings> {
  const { data } = await apiClient.get<OAuthSleeperSettings>('/admin/oauth-sleeper/settings')
  return data
}

export async function updateSettings(
  payload: OAuthSleeperSettings
): Promise<OAuthSleeperSettings> {
  const { data } = await apiClient.put<OAuthSleeperSettings>('/admin/oauth-sleeper/settings', payload)
  return data
}

export async function scanOnce(): Promise<OAuthSleeperScanResult> {
  const { data } = await apiClient.post<OAuthSleeperScanResult>('/admin/oauth-sleeper/scan-once')
  return data
}

export async function listEvents(
  params: OAuthSleeperEventsParams = {}
): Promise<PaginatedResponse<OAuthSleeperEvent>> {
  const { data } = await apiClient.get<PaginatedResponse<OAuthSleeperEvent>>(
    '/admin/oauth-sleeper/events',
    { params }
  )
  return data
}

export const oauthSleeperAPI = {
  getStatus,
  getSettings,
  updateSettings,
  scanOnce,
  listEvents,
}

export default oauthSleeperAPI
