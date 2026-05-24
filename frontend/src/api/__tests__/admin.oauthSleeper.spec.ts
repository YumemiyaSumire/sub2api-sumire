import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
  },
}))

import oauthSleeperAPI, { type OAuthSleeperSettings } from '@/api/admin/oauthSleeper'

describe('admin oauthSleeper api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
  })

  it('fetches status and settings from the admin oauth sleeper endpoints', async () => {
    get
      .mockResolvedValueOnce({ data: { enabled: false, sleeping_accounts: [] } })
      .mockResolvedValueOnce({ data: { enabled: false, threshold_percent: 95 } })

    await expect(oauthSleeperAPI.getStatus()).resolves.toEqual({
      enabled: false,
      sleeping_accounts: [],
    })
    await expect(oauthSleeperAPI.getSettings()).resolves.toEqual({
      enabled: false,
      threshold_percent: 95,
    })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/oauth-sleeper/status')
    expect(get).toHaveBeenNthCalledWith(2, '/admin/oauth-sleeper/settings')
  })

  it('saves settings and runs manual scans with backend-compatible routes', async () => {
    const payload: OAuthSleeperSettings = {
      enabled: true,
      threshold_percent: 96,
      scan_interval_seconds: 300,
      max_sleep_per_scan: 2,
      include_openai: true,
      include_anthropic: false,
      group_ids: [1],
    }
    put.mockResolvedValue({ data: payload })
    post.mockResolvedValue({ data: { scanned: 4, triggered: 1, events: [] } })

    await expect(oauthSleeperAPI.updateSettings(payload)).resolves.toEqual(payload)
    await expect(oauthSleeperAPI.scanOnce()).resolves.toEqual({
      scanned: 4,
      triggered: 1,
      events: [],
    })

    expect(put).toHaveBeenCalledWith('/admin/oauth-sleeper/settings', payload)
    expect(post).toHaveBeenCalledWith('/admin/oauth-sleeper/scan-once')
  })

  it('lists events with pagination params', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0, page: 2, page_size: 10, pages: 0 } })

    await expect(oauthSleeperAPI.listEvents({ page: 2, page_size: 10 })).resolves.toEqual({
      items: [],
      total: 0,
      page: 2,
      page_size: 10,
      pages: 0,
    })

    expect(get).toHaveBeenCalledWith('/admin/oauth-sleeper/events', {
      params: { page: 2, page_size: 10 },
    })
  })
})
