<template>
  <BaseDialog
    :show="show"
    :title="t('admin.scheduledGroupTests.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="max-w-2xl space-y-1">
          <p class="text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.scheduledGroupTests.description') }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.scheduledGroupTests.logsOnly') }}
          </p>
        </div>
        <button
          type="button"
          data-test="show-add-form"
          class="btn btn-primary flex items-center gap-1.5 text-sm"
          @click="showAddForm = !showAddForm"
        >
          <Icon name="plus" size="sm" :stroke-width="2" />
          {{ t('admin.scheduledGroupTests.addPlan') }}
        </button>
      </div>

      <div
        v-if="showAddForm"
        class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/20"
      >
        <div class="mb-3 text-sm font-medium text-gray-800 dark:text-gray-200">
          {{ t('admin.scheduledGroupTests.addPlan') }}
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div class="space-y-1.5">
            <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledGroupTests.group') }}
            </label>
            <Select
              v-model="newPlan.group_id"
              :options="groupOptions"
              :placeholder="t('admin.scheduledGroupTests.groupPlaceholder')"
              :empty-text="t('admin.scheduledGroupTests.noGroups')"
              :searchable="groupOptions.length > 5"
            />
          </div>
          <Input
            v-model="newPlan.account_name_filter"
            :label="t('admin.scheduledGroupTests.accountNameFilter')"
            :placeholder="t('admin.scheduledGroupTests.accountNameFilterPlaceholder')"
            :hint="t('admin.scheduledGroupTests.emptyFilterMeansAll')"
            autocomplete="off"
          />
          <div class="rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-800">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.scheduledGroupTests.fixedModel') }}
            </div>
            <div class="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
              gpt-5.5
            </div>
          </div>
          <div class="flex items-center">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="newPlan.enabled" />
              {{ newPlan.enabled ? t('admin.scheduledGroupTests.enabled') : t('admin.scheduledGroupTests.disabled') }}
            </label>
          </div>
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button
            type="button"
            class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
            @click="cancelCreate"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            data-test="create-plan"
            class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="!canCreate || creating"
            @click="handleCreate"
          >
            <Icon v-if="creating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            {{ creating ? t('admin.scheduledGroupTests.saving') : t('admin.scheduledGroupTests.save') }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-10">
        <Icon name="refresh" size="md" class="animate-spin text-gray-400" :stroke-width="2" />
        <span class="ml-2 text-sm text-gray-500">{{ t('common.loading') }}...</span>
      </div>

      <div
        v-else-if="plans.length === 0"
        class="rounded-xl border border-dashed border-gray-300 py-10 text-center dark:border-dark-600"
      >
        <Icon name="calendar" size="lg" class="mx-auto mb-2 text-gray-400" :stroke-width="1.5" />
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.scheduledGroupTests.noPlans') }}
        </p>
      </div>

      <div v-else class="space-y-3">
        <div
          v-for="plan in plans"
          :key="plan.id"
          class="rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                  {{ groupName(plan.group_id) }}
                </div>
                <span class="rounded-full bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ plan.model_id }}
                </span>
                <span
                  :class="[
                    'rounded-full px-2 py-0.5 text-xs font-medium',
                    plan.enabled
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
                      : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
                  ]"
                >
                  {{ plan.enabled ? t('admin.scheduledGroupTests.enabled') : t('admin.scheduledGroupTests.disabled') }}
                </span>
              </div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.scheduledGroupTests.accountNameFilter') }}:
                <span class="font-medium text-gray-700 dark:text-gray-300">
                  {{ plan.account_name_filter || t('admin.scheduledGroupTests.wholeGroup') }}
                </span>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3 text-right text-xs text-gray-500 dark:text-gray-400">
              <div>
                <div>{{ t('admin.scheduledGroupTests.lastRun') }}</div>
                <div class="mt-0.5 text-gray-700 dark:text-gray-300">
                  {{ formatPlanDate(plan.last_run_at) }}
                </div>
              </div>
              <div>
                <div>{{ t('admin.scheduledGroupTests.nextRun') }}</div>
                <div class="mt-0.5 text-gray-700 dark:text-gray-300">
                  {{ formatPlanDate(plan.next_run_at) }}
                </div>
              </div>
            </div>

            <div class="flex items-center gap-1">
              <Toggle
                :model-value="plan.enabled"
                @update:model-value="(enabled: boolean) => handleToggleEnabled(plan, enabled)"
              />
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-blue-50 hover:text-blue-500 dark:hover:bg-blue-900/20"
                :title="t('admin.scheduledGroupTests.editPlan')"
                @click="startEdit(plan)"
              >
                <Icon name="edit" size="sm" :stroke-width="2" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
                :title="t('admin.scheduledGroupTests.deletePlan')"
                @click="confirmDeletePlan(plan)"
              >
                <Icon name="trash" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>

          <div
            v-if="editingPlanId === plan.id"
            class="border-t border-blue-100 bg-blue-50/50 px-4 py-3 dark:border-blue-900 dark:bg-blue-900/10"
          >
            <div class="mb-3 text-sm font-medium text-gray-800 dark:text-gray-200">
              {{ t('admin.scheduledGroupTests.editPlan') }}
            </div>
            <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
              <div class="space-y-1.5">
                <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledGroupTests.group') }}
                </label>
                <Select
                  v-model="editForm.group_id"
                  :options="groupOptions"
                  :placeholder="t('admin.scheduledGroupTests.groupPlaceholder')"
                  :empty-text="t('admin.scheduledGroupTests.noGroups')"
                  :searchable="groupOptions.length > 5"
                />
              </div>
              <Input
                v-model="editForm.account_name_filter"
                :label="t('admin.scheduledGroupTests.accountNameFilter')"
                :placeholder="t('admin.scheduledGroupTests.accountNameFilterPlaceholder')"
                :hint="t('admin.scheduledGroupTests.emptyFilterMeansAll')"
                autocomplete="off"
              />
              <div class="rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-800">
                <div class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.scheduledGroupTests.fixedModel') }}
                </div>
                <div class="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
                  gpt-5.5
                </div>
              </div>
              <div class="flex items-center">
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <Toggle v-model="editForm.enabled" />
                  {{ editForm.enabled ? t('admin.scheduledGroupTests.enabled') : t('admin.scheduledGroupTests.disabled') }}
                </label>
              </div>
            </div>
            <div class="mt-4 flex justify-end gap-2">
              <button
                type="button"
                class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
                @click="cancelEdit"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="!canEdit || updating"
                @click="handleEdit"
              >
                <Icon v-if="updating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
                {{ updating ? t('admin.scheduledGroupTests.saving') : t('admin.scheduledGroupTests.save') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('admin.scheduledGroupTests.deletePlan')"
      :message="t('admin.scheduledGroupTests.confirmDelete')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { AdminGroup, ScheduledGroupTestPlan } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  show: boolean
  groups: AdminGroup[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const loading = ref(false)
const creating = ref(false)
const updating = ref(false)
const plans = ref<ScheduledGroupTestPlan[]>([])
const showAddForm = ref(false)
const editingPlanId = ref<number | null>(null)
const showDeleteConfirm = ref(false)
const deletingPlan = ref<ScheduledGroupTestPlan | null>(null)

const newPlan = reactive({
  group_id: null as number | null,
  account_name_filter: '',
  enabled: true
})

const editForm = reactive({
  group_id: null as number | null,
  account_name_filter: '',
  enabled: true
})

const groupOptions = computed(() =>
  props.groups
    .filter((group) => group.id > 0)
    .map((group) => ({
      value: group.id,
      label: group.name
    }))
)

const canCreate = computed(() => typeof newPlan.group_id === 'number' && newPlan.group_id > 0)
const canEdit = computed(() => typeof editForm.group_id === 'number' && editForm.group_id > 0)

const resetNewPlan = () => {
  newPlan.group_id = groupOptions.value.length === 1 ? Number(groupOptions.value[0].value) : null
  newPlan.account_name_filter = ''
  newPlan.enabled = true
}

const loadPlans = async () => {
  loading.value = true
  try {
    plans.value = await adminAPI.scheduledGroupTests.list()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.scheduledGroupTests.loadFailed'))
  } finally {
    loading.value = false
  }
}

const cancelCreate = () => {
  showAddForm.value = false
  resetNewPlan()
}

const handleCreate = async () => {
  if (!canCreate.value || newPlan.group_id == null) return
  creating.value = true
  try {
    await adminAPI.scheduledGroupTests.create({
      group_id: newPlan.group_id,
      account_name_filter: newPlan.account_name_filter.trim(),
      enabled: newPlan.enabled
    })
    appStore.showSuccess(t('admin.scheduledGroupTests.createSuccess'))
    showAddForm.value = false
    resetNewPlan()
    await loadPlans()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.scheduledGroupTests.createFailed'))
  } finally {
    creating.value = false
  }
}

const handleToggleEnabled = async (plan: ScheduledGroupTestPlan, enabled: boolean) => {
  try {
    const updated = await adminAPI.scheduledGroupTests.update(plan.id, { enabled })
    replacePlan(updated)
    appStore.showSuccess(t('admin.scheduledGroupTests.updateSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.scheduledGroupTests.updateFailed'))
  }
}

const startEdit = (plan: ScheduledGroupTestPlan) => {
  editingPlanId.value = plan.id
  editForm.group_id = plan.group_id
  editForm.account_name_filter = plan.account_name_filter || ''
  editForm.enabled = plan.enabled
}

const cancelEdit = () => {
  editingPlanId.value = null
}

const handleEdit = async () => {
  if (!editingPlanId.value || !canEdit.value || editForm.group_id == null) return
  updating.value = true
  try {
    const updated = await adminAPI.scheduledGroupTests.update(editingPlanId.value, {
      group_id: editForm.group_id,
      account_name_filter: editForm.account_name_filter.trim(),
      enabled: editForm.enabled
    })
    replacePlan(updated)
    appStore.showSuccess(t('admin.scheduledGroupTests.updateSuccess'))
    editingPlanId.value = null
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.scheduledGroupTests.updateFailed'))
  } finally {
    updating.value = false
  }
}

const confirmDeletePlan = (plan: ScheduledGroupTestPlan) => {
  deletingPlan.value = plan
  showDeleteConfirm.value = true
}

const handleDelete = async () => {
  if (!deletingPlan.value) return
  try {
    await adminAPI.scheduledGroupTests.delete(deletingPlan.value.id)
    plans.value = plans.value.filter((plan) => plan.id !== deletingPlan.value!.id)
    appStore.showSuccess(t('admin.scheduledGroupTests.deleteSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.scheduledGroupTests.deleteFailed'))
  } finally {
    showDeleteConfirm.value = false
    deletingPlan.value = null
  }
}

const replacePlan = (updated: ScheduledGroupTestPlan) => {
  const index = plans.value.findIndex((plan) => plan.id === updated.id)
  if (index !== -1) {
    plans.value[index] = updated
  }
}

const groupName = (groupId: number) => {
  return props.groups.find((group) => group.id === groupId)?.name ?? `#${groupId}`
}

const formatPlanDate = (value: string | null | undefined) => {
  return value ? formatDateTime(value) : t('common.time.never')
}

watch(
  () => props.show,
  async (visible) => {
    if (visible) {
      resetNewPlan()
      editingPlanId.value = null
      showDeleteConfirm.value = false
      await loadPlans()
    } else {
      plans.value = []
      showAddForm.value = false
      editingPlanId.value = null
      showDeleteConfirm.value = false
      deletingPlan.value = null
    }
  },
  { immediate: true }
)
</script>
