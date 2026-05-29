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
  listEvents,
  getAllGroups,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getStatus: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
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
      t: (key: string, params?: Record<string, string | number>) => {
        const messages: Record<string, string> = {
          'admin.oauthSleeper.lastScanMeta': 'Checked {scanned}, triggered {triggered}',
          'admin.oauthSleeper.sleepingCount': '{count} sleeping',
        }
        return (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`))
      },
    }),
  }
})

const baseSettings = (): OAuthSleeperSettings => ({
  enabled: false,
  threshold_percent: 90,
  group_threshold_percent: {},
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

describe('admin OAuthSleeperView', () => {
  beforeEach(() => {
    getStatus.mockReset()
    getSettings.mockReset()
    updateSettings.mockReset()
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
    expect(wrapper.text()).not.toContain('admin.oauthSleeper.scanOnce')
  })

  it('renders sleeping accounts as cards', async () => {
    getStatus.mockResolvedValueOnce({
      ...baseStatus(),
      sleeping_accounts: [
        {
          account_id: 9,
          account_name: 'sleeping-openai',
          platform: 'openai',
          rate_limit_reset_at: '2026-05-24T01:00:00Z',
          remaining_seconds: 3600,
        },
        {
          account_id: 10,
          account_name: 'sleeping-claude',
          platform: 'anthropic',
          rate_limit_reset_at: '2026-05-24T02:00:00Z',
          remaining_seconds: 7200,
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('sleeping-openai')
    expect(wrapper.text()).toContain('sleeping-claude')
    expect(wrapper.findAll('article')).toHaveLength(2)
  })

  it('saves settings payload from the form', async () => {
    const wrapper = mountView()
    await flushPromises()

    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[0].setValue('96')
    await wrapper.findAll('button[role="switch"]')[0].trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith({
      enabled: true,
      threshold_percent: 96,
      group_threshold_percent: {},
      include_openai: true,
      include_anthropic: true,
      group_ids: [1],
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.oauthSleeper.saved')
  })

  it('saves selected group threshold overrides and drops unselected values', async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettings(),
      group_ids: [1],
      group_threshold_percent: { 1: 88, 2: 80 },
    })
    getStatus.mockResolvedValueOnce({
      ...baseStatus(),
      group_ids: [1],
      group_threshold_percent: { 1: 88, 2: 80 },
    })
    const wrapper = mountView()
    await flushPromises()

    const overrideInput = wrapper
      .findAll('input[type="number"]')
      .find((input) => input.attributes('value') === '88')
    if (!overrideInput) throw new Error('group threshold input not found')
    await overrideInput.setValue('87')
    await wrapper.findAll('button[role="switch"]')[0].trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
      group_ids: [1],
      group_threshold_percent: { 1: 87 },
    }))
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
