<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.batchTestTitle')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-1.5">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.batchTestGroup') }}
          </label>
          <Select
            v-model="selectedGroupId"
            :options="groupOptions"
            :disabled="running"
            :placeholder="t('admin.accounts.batchTestGroupPlaceholder')"
            :empty-text="t('admin.accounts.batchTestNoGroups')"
          />
        </div>
        <div class="space-y-2">
          <Input
            v-model="modelId"
            :label="t('admin.accounts.batchTestModel')"
            :placeholder="t('admin.accounts.batchTestModelPlaceholder')"
            :disabled="running"
            autocomplete="off"
            @enter="startBatchTest"
          />
          <div class="flex flex-wrap gap-2" :aria-label="t('admin.accounts.batchTestModelPresets')">
            <button
              v-for="preset in modelPresets"
              :key="preset.id"
              type="button"
              :data-test="`preset-model-${preset.id}`"
              class="rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-600 transition hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-200"
              :class="modelId.trim() === preset.id ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-600 dark:bg-primary-900/30 dark:text-primary-200' : ''"
              :disabled="running"
              @click="applyModelPreset(preset.id)"
            >
              {{ preset.label }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="running"
        class="flex items-center gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-700 dark:border-blue-800/60 dark:bg-blue-900/20 dark:text-blue-200"
      >
        <Icon name="refresh" size="sm" class="animate-spin" />
        <span>{{ t('admin.accounts.batchTesting') }}</span>
      </div>

      <div
        v-if="errorMessage"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-200"
      >
        {{ errorMessage }}
      </div>

      <div
        v-if="result && !running"
        class="flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-900/20 dark:text-emerald-200"
      >
        <Icon name="check" size="sm" />
        <span>{{ t('admin.accounts.batchTestDone') }}</span>
      </div>

      <div v-if="result" class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
            <div class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.batchTestTotal') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
              {{ result.total }}
            </div>
          </div>
          <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 dark:border-emerald-800/60 dark:bg-emerald-900/20">
            <div class="text-xs font-medium uppercase text-emerald-700 dark:text-emerald-300">
              {{ t('admin.accounts.batchTestSuccess') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-emerald-700 dark:text-emerald-200">
              {{ result.success }}
            </div>
          </div>
          <div class="rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800/60 dark:bg-red-900/20">
            <div class="text-xs font-medium uppercase text-red-700 dark:text-red-300">
              {{ t('admin.accounts.batchTestFailedCount') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-red-700 dark:text-red-200">
              {{ result.failed }}
            </div>
          </div>
        </div>

        <div
          v-if="result.total === 0"
          class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-6 text-center text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-400"
        >
          {{ t('admin.accounts.batchTestEmptyGroup') }}
        </div>

        <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
          <div class="max-h-[360px] overflow-y-auto divide-y divide-gray-100 dark:divide-gray-700">
            <div
              v-for="item in result.results"
              :key="item.account_id"
              class="bg-white p-4 dark:bg-gray-800"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-900 dark:text-gray-100">
                    {{ item.account_name }}
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    ID {{ item.account_id }}
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    :class="[
                      'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold',
                      item.status === 'success'
                        ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
                        : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
                    ]"
                  >
                    <Icon :name="item.status === 'success' ? 'check' : 'x'" size="xs" />
                    {{ item.status === 'success' ? t('admin.accounts.batchTestStatusSuccess') : t('admin.accounts.batchTestStatusFailed') }}
                  </span>
                  <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                    <Icon name="clock" size="xs" />
                    {{ formatLatency(item.latency_ms) }}
                  </span>
                </div>
              </div>

              <div
                v-if="item.error_message"
                class="mt-3 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-200"
              >
                <div class="mb-1 font-semibold">{{ t('admin.accounts.batchTestErrorDetails') }}</div>
                <pre class="whitespace-pre-wrap break-words font-mono">{{ item.error_message }}</pre>
              </div>
              <div
                v-else-if="item.response_text"
                class="mt-3 rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-gray-900/40 dark:text-gray-300"
              >
                <div class="mb-1 font-semibold">{{ t('admin.accounts.batchTestResponseText') }}</div>
                <pre class="line-clamp-4 whitespace-pre-wrap break-words font-mono">{{ item.response_text }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          @click="handleClose"
        >
          {{ t('common.close') }}
        </button>
        <button
          class="flex items-center gap-2 rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:bg-primary-400"
          :disabled="!canStart"
          @click="startBatchTest"
        >
          <Icon v-if="running" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="play" size="sm" />
          <span>{{ running ? t('admin.accounts.batchTesting') : t('admin.accounts.batchTestStart') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type { BatchTestAccountsResponse } from '@/api/admin/accounts'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  groups: AdminGroup[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const selectedGroupId = ref<number | null>(null)
const modelId = ref('')
const running = ref(false)
const result = ref<BatchTestAccountsResponse | null>(null)
const errorMessage = ref('')
const modelPresets = [
  { id: 'claude-sonnet-4-5', label: 'Claude Sonnet 4.5' },
  { id: 'gpt-5.4', label: 'GPT-5.4' },
  { id: 'gpt-5.4-mini', label: 'GPT-5.4 Mini' },
  { id: 'gemini-2.0-flash', label: 'Gemini 2.0 Flash' }
]

const groupOptions = computed(() =>
  props.groups
    .filter((group) => group.id > 0)
    .map((group) => ({
      value: group.id,
      label: group.name
    }))
)

const canStart = computed(() =>
  !running.value &&
  typeof selectedGroupId.value === 'number' &&
  selectedGroupId.value > 0 &&
  modelId.value.trim().length > 0
)

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    if (running.value) return
    selectedGroupId.value = groupOptions.value.length === 1 ? Number(groupOptions.value[0].value) : null
    modelId.value = ''
    running.value = false
    result.value = null
    errorMessage.value = ''
  }
)

const handleClose = () => {
  emit('close')
}

const applyModelPreset = (presetModelId: string) => {
  if (running.value) return
  modelId.value = presetModelId
}

const startBatchTest = async () => {
  if (!canStart.value || selectedGroupId.value == null) return
  running.value = true
  result.value = null
  errorMessage.value = ''
  try {
    result.value = await adminAPI.accounts.batchTestByGroup(selectedGroupId.value, modelId.value.trim())
  } catch (error: any) {
    errorMessage.value = error?.message || t('admin.accounts.batchTestFailed')
  } finally {
    running.value = false
  }
}

const formatLatency = (latencyMs: number | undefined) => {
  if (typeof latencyMs !== 'number' || !Number.isFinite(latencyMs) || latencyMs < 0) {
    return '-'
  }
  return `${latencyMs}ms`
}
</script>
