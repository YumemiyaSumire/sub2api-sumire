import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ScheduledGroupTestsPanel from '../ScheduledGroupTestsPanel.vue'

const { listPlans, createPlan, updatePlan, deletePlan, showError, showSuccess } = vi.hoisted(() => ({
  listPlans: vi.fn(),
  createPlan: vi.fn(),
  updatePlan: vi.fn(),
  deletePlan: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    scheduledGroupTests: {
      list: listPlans,
      create: createPlan,
      update: updatePlan,
      delete: deletePlan
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string | null | undefined) => value ?? ''
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `
    <select
      data-test="group-select"
      :value="modelValue ?? ''"
      @change="$emit('update:modelValue', Number($event.target.value))"
    >
      <option value=""></option>
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `
}

const InputStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: `
    <input
      data-test="account-name-filter"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
    />
  `
}

const ToggleStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: `
    <button
      type="button"
      data-test="toggle"
      :data-value="String(modelValue)"
      @click="$emit('update:modelValue', !modelValue)"
    >
      toggle
    </button>
  `
}

function mountPanel(props = {}) {
  return mount(ScheduledGroupTestsPanel, {
    props: {
      show: true,
      groups: [
        { id: 1, name: 'OpenAI Group' },
        { id: 2, name: 'Claude Group' }
      ],
      ...props
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: {
          props: ['show'],
          emits: ['confirm', 'cancel'],
          template: '<div v-if="show" data-test="confirm-dialog"><button data-test="confirm-delete" @click="$emit(\'confirm\')">confirm</button></div>'
        },
        Select: SelectStub,
        Input: InputStub,
        Toggle: ToggleStub,
        Icon: true
      }
    }
  })
}

describe('ScheduledGroupTestsPanel', () => {
  beforeEach(() => {
    listPlans.mockReset()
    createPlan.mockReset()
    updatePlan.mockReset()
    deletePlan.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listPlans.mockResolvedValue([])
    createPlan.mockResolvedValue({
      id: 1,
      group_id: 1,
      account_name_filter: '',
      model_id: 'gpt-5.5',
      enabled: true,
      last_run_at: null,
      next_run_at: '2026-06-07T10:00:00Z',
      created_at: '2026-06-07T10:00:00Z',
      updated_at: '2026-06-07T10:00:00Z'
    })
    updatePlan.mockImplementation((_id: number, req: Record<string, unknown>) =>
      Promise.resolve({
        id: 7,
        group_id: 1,
        account_name_filter: 'beta',
        model_id: 'gpt-5.5',
        enabled: req.enabled ?? true,
        last_run_at: '2026-06-07T09:00:00Z',
        next_run_at: '2026-06-07T10:00:00Z',
        created_at: '2026-06-07T08:00:00Z',
        updated_at: '2026-06-07T09:30:00Z'
      })
    )
    deletePlan.mockResolvedValue(undefined)
  })

  it('loads plans when opened and disables save until a group is selected', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(listPlans).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-test="show-add-form"]').trigger('click')

    const saveButton = wrapper.get('[data-test="create-plan"]')
    expect(saveButton.attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="group-select"]').setValue('1')
    expect(saveButton.attributes('disabled')).toBeUndefined()
  })

  it('allows empty account name filter and submits the whole group plan', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="show-add-form"]').trigger('click')
    await wrapper.get('[data-test="group-select"]').setValue('1')
    await wrapper.get('[data-test="create-plan"]').trigger('click')
    await flushPromises()

    expect(createPlan).toHaveBeenCalledWith({
      group_id: 1,
      account_name_filter: '',
      enabled: true
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.scheduledGroupTests.createSuccess')
  })

  it('renders plan list with group, filter, model, and run times', async () => {
    listPlans.mockResolvedValue([
      {
        id: 7,
        group_id: 1,
        account_name_filter: 'beta',
        model_id: 'gpt-5.5',
        enabled: true,
        last_run_at: '2026-06-07T09:00:00Z',
        next_run_at: '2026-06-07T10:00:00Z',
        created_at: '2026-06-07T08:00:00Z',
        updated_at: '2026-06-07T09:30:00Z'
      }
    ])

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('OpenAI Group')
    expect(wrapper.text()).toContain('beta')
    expect(wrapper.text()).toContain('gpt-5.5')
    expect(wrapper.text()).toContain('2026-06-07T09:00:00Z')
    expect(wrapper.text()).toContain('2026-06-07T10:00:00Z')
  })

  it('updates enabled status from the plan toggle', async () => {
    listPlans.mockResolvedValue([
      {
        id: 7,
        group_id: 1,
        account_name_filter: '',
        model_id: 'gpt-5.5',
        enabled: true,
        last_run_at: null,
        next_run_at: '2026-06-07T10:00:00Z',
        created_at: '2026-06-07T08:00:00Z',
        updated_at: '2026-06-07T09:30:00Z'
      }
    ])

    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="toggle"]').trigger('click')
    await flushPromises()

    expect(updatePlan).toHaveBeenCalledWith(7, { enabled: false })
    expect(showSuccess).toHaveBeenCalledWith('admin.scheduledGroupTests.updateSuccess')
  })
})
