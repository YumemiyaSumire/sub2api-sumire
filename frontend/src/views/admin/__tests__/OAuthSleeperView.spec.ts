import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import OAuthSleeperView from '../OAuthSleeperView.vue'
import type {
  OAuthSleeperSettings,
  OAuthSleeperStatus,
} from '@/api/admin/oauthSleeper'

const {
  getStatus,
  getSettings,
  updateSettings,
  scanOnce,
  listEvents,
  getAllGroups,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getStatus: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  scanOnce: vi.fn(),
  listEvents: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    oauthSleeper: {
      getStatus,
      getSettings,
      updateSettings,
      scanOnce,
      listEvents,
    },
    groups: {
      getAll: getAllGroups,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_err: unknown, fallback: string) => fallback,
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string | null | undefined) => value ?? '',
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) =>
        key.replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`)),
    }),
  }
})

const baseSettings = (): OAuthSleeperSettings => ({
  enabled: false,
  threshold_percent: 95,
  scan_interval_seconds: 300,
  max_sleep_per_scan: 3,
  include_openai: true,
  include_anthropic: true,
  group_ids: [1],
})

const baseStatus = (): OAuthSleeperStatus => ({
  ...baseSettings(),
  last_scan_at: '2026-05-24T00:00:00Z',
  last_scanned: 2,
  last_triggered: 1,
  sleeping_accounts: [],
})

function mountView(): VueWrapper {
  return mount(OAuthSleeperView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
      },
    },
  })
}

function findButtonByText(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

describe('admin OAuthSleeperView', () => {
  beforeEach(() => {
    getStatus.mockReset()
    getSettings.mockReset()
    updateSettings.mockReset()
    scanOnce.mockReset()
    listEvents.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getSettings.mockResolvedValue(baseSettings())
    getStatus.mockResolvedValue(baseStatus())
    getAllGroups.mockImplementation(async (platform?: string) => {
      if (platform === 'openai') {
        return [{ id: 1, name: 'OpenAI group', platform: 'openai', rate_multiplier: 1, account_count: 2 }]
      }
      if (platform === 'anthropic') {
        return [{ id: 2, name: 'Claude group', platform: 'anthropic', rate_multiplier: 1, account_count: 1 }]
      }
      return []
    })
    listEvents.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    updateSettings.mockImplementation(async (payload: OAuthSleeperSettings) => payload)
    scanOnce.mockResolvedValue({ scanned: 2, triggered: 1, events: [] })
  })

  it('loads status, settings and empty event state', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(1)
    expect(getStatus).toHaveBeenCalledTimes(1)
    expect(getAllGroups).toHaveBeenCalledWith('openai')
    expect(getAllGroups).toHaveBeenCalledWith('anthropic')
    expect(listEvents).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(wrapper.text()).toContain('OpenAI group')
    expect(wrapper.text()).toContain('admin.oauthSleeper.noEvents')
    expect(wrapper.text()).toContain('admin.oauthSleeper.noSleepingAccounts')
  })

  it('saves settings payload from the form', async () => {
    const wrapper = mountView()
    await flushPromises()

    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[0].setValue('96')
    await inputs[1].setValue('600')
    await inputs[2].setValue('2')
    await wrapper.findAll('button[role="switch"]')[0].trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith({
      enabled: true,
      threshold_percent: 96,
      scan_interval_seconds: 600,
      max_sleep_per_scan: 2,
      include_openai: true,
      include_anthropic: true,
      group_ids: [1],
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.oauthSleeper.saved')
  })

  it('refreshes status and events after a manual scan', async () => {
    const wrapper = mountView()
    await flushPromises()

    getStatus.mockClear()
    listEvents.mockClear()

    await findButtonByText(wrapper, 'admin.oauthSleeper.scanOnce').trigger('click')
    await flushPromises()

    expect(scanOnce).toHaveBeenCalledTimes(1)
    expect(getStatus).toHaveBeenCalledTimes(1)
    expect(listEvents).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(showSuccess).toHaveBeenCalledWith('admin.oauthSleeper.scanSuccess')
  })

  it('shows an error when saving with no platform enabled', async () => {
    const wrapper = mountView()
    await flushPromises()

    const switches = wrapper.findAll('button[role="switch"]')
    await switches[1].trigger('click')
    await switches[2].trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.oauthSleeper.platformRequired')
  })

  it('shows an error when enabling with no group selected', async () => {
    getSettings.mockResolvedValueOnce({ ...baseSettings(), enabled: false, group_ids: [] })
    getStatus.mockResolvedValueOnce({ ...baseStatus(), enabled: false, group_ids: [] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button[role="switch"]')[0].trigger('click')
    const groupCheckbox = wrapper.find('input[type="checkbox"][value="1"]')
    if (groupCheckbox.exists()) {
      await groupCheckbox.setValue(false)
    }
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.oauthSleeper.groupRequired')
  })
})
