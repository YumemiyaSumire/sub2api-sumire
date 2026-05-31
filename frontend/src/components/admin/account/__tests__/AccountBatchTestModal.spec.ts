import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountBatchTestModal from '../AccountBatchTestModal.vue'

const { batchTestByGroup } = vi.hoisted(() => ({
  batchTestByGroup: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      batchTestByGroup
    }
  }
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
  emits: ['update:modelValue', 'enter'],
  template: `
    <input
      data-test="model-input"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
      @keyup.enter="$emit('enter')"
    />
  `
}

function mountModal(props = {}) {
  return mount(AccountBatchTestModal, {
    props: {
      show: true,
      groups: [
        { id: 1, name: 'Anthropic Group' },
        { id: 2, name: 'OpenAI Group' }
      ],
      ...props
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: SelectStub,
        Input: InputStub,
        Icon: true
      }
    }
  })
}

describe('AccountBatchTestModal', () => {
  beforeEach(() => {
    batchTestByGroup.mockReset()
    batchTestByGroup.mockResolvedValue({
      group_id: 1,
      model_id: 'claude-sonnet-4-5',
      total: 2,
      success: 1,
      failed: 1,
      results: [
        {
          account_id: 10,
          account_name: 'Alpha',
          status: 'success',
          latency_ms: 120,
          response_text: 'ok'
        },
        {
          account_id: 11,
          account_name: 'Beta',
          status: 'failed',
          latency_ms: 30,
          error_message: 'bad token'
        }
      ]
    })
  })

  it('disables start until a group and model are selected', async () => {
    const wrapper = mountModal()

    const startButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.batchTestStart')
    )
    expect(startButton?.attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="group-select"]').setValue('1')
    expect(startButton?.attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="model-input"]').setValue('claude-sonnet-4-5')
    expect(startButton?.attributes('disabled')).toBeUndefined()
  })

  it('submits selected group and model, then renders summary and account results', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-test="group-select"]').setValue('1')
    await wrapper.get('[data-test="model-input"]').setValue('claude-sonnet-4-5')

    const startButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.batchTestStart')
    )
    await startButton!.trigger('click')
    await flushPromises()

    expect(batchTestByGroup).toHaveBeenCalledWith(1, 'claude-sonnet-4-5')
    expect(wrapper.text()).toContain('Alpha')
    expect(wrapper.text()).toContain('Beta')
    expect(wrapper.text()).toContain('bad token')
    expect(wrapper.text()).toContain('120ms')
    expect(wrapper.text()).toContain('admin.accounts.batchTestDone')
  })

  it('offers gpt-5.4-mini as a preset test model', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-test="group-select"]').setValue('2')
    await wrapper.get('[data-test="preset-model-gpt-5.4-mini"]').trigger('click')

    expect((wrapper.get('[data-test="model-input"]').element as HTMLInputElement).value).toBe('gpt-5.4-mini')

    const startButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.batchTestStart')
    )
    await startButton!.trigger('click')
    await flushPromises()

    expect(batchTestByGroup).toHaveBeenCalledWith(2, 'gpt-5.4-mini')
  })

  it('renders empty group message', async () => {
    batchTestByGroup.mockResolvedValue({
      group_id: 1,
      model_id: 'claude-sonnet-4-5',
      total: 0,
      success: 0,
      failed: 0,
      results: []
    })
    const wrapper = mountModal()

    await wrapper.get('[data-test="group-select"]').setValue('1')
    await wrapper.get('[data-test="model-input"]').setValue('claude-sonnet-4-5')
    const startButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.batchTestStart')
    )
    await startButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.batchTestEmptyGroup')
  })

  it('does not reset running state when closed and reopened during a request', async () => {
    let resolveRequest!: (value: any) => void
    batchTestByGroup.mockReturnValue(new Promise(resolve => {
      resolveRequest = resolve
    }))
    const wrapper = mountModal()

    await wrapper.get('[data-test="group-select"]').setValue('1')
    await wrapper.get('[data-test="model-input"]').setValue('claude-sonnet-4-5')
    const startButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.batchTestStart')
    )
    await startButton!.trigger('click')

    expect(batchTestByGroup).toHaveBeenCalledTimes(1)
    expect(startButton!.attributes('disabled')).toBeDefined()

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(startButton!.attributes('disabled')).toBeDefined()
    await startButton!.trigger('click')
    expect(batchTestByGroup).toHaveBeenCalledTimes(1)

    resolveRequest({
      group_id: 1,
      model_id: 'claude-sonnet-4-5',
      total: 0,
      success: 0,
      failed: 0,
      results: []
    })
    await flushPromises()
  })
})
