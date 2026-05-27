<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary inline-flex items-center gap-2"
            :disabled="loading || eventsLoading"
            @click="refreshAll"
          >
            <Icon name="refresh" size="sm" :class="loading || eventsLoading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
          <button
            type="button"
            class="btn btn-primary inline-flex items-center gap-2"
            :disabled="scanning"
            @click="runScanOnce"
          >
            <Icon name="play" size="sm" :class="scanning ? 'animate-spin' : ''" />
            {{ scanning ? t('admin.oauthSleeper.scanning') : t('admin.oauthSleeper.scanOnce') }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div
            v-for="item in overviewItems"
            :key="item.key"
            class="rounded-lg border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex min-w-0 items-center gap-3">
              <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg" :class="item.iconClass">
                <Icon :name="item.icon" size="sm" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex min-w-0 items-center justify-between gap-2">
                  <p class="truncate text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</p>
                  <span
                    v-if="item.badge"
                    class="inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="item.badgeClass"
                  >
                    {{ item.badge }}
                  </span>
                </div>
                <div class="mt-1 min-w-0">
                  <p class="truncate text-xl font-semibold leading-7 text-gray-900 dark:text-white">{{ item.value }}</p>
                  <p v-if="item.meta" class="mt-0.5 text-xs leading-4 text-gray-500 dark:text-gray-400">{{ item.meta }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="isAccelerated"
          class="rounded-lg border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800 dark:border-sky-900/50 dark:bg-sky-900/20 dark:text-sky-200"
        >
          <div class="flex items-start gap-3">
            <Icon name="bolt" size="sm" class="mt-0.5 flex-shrink-0" />
            <div>
              <p class="font-medium">{{ t('admin.oauthSleeper.acceleratedNoticeTitle') }}</p>
              <p class="mt-0.5">{{ accelerationMetaText }}</p>
            </div>
          </div>
        </div>

        <div
          v-if="status?.last_error"
          class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
        >
          {{ t('admin.oauthSleeper.lastError', { error: status.last_error }) }}
        </div>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,420px)_1fr]">
          <div class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.settingsTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.settingsHint') }}</p>
            </div>

            <form class="space-y-5 p-6" @submit.prevent="saveSettings">
              <div class="flex items-center justify-between gap-4 rounded-lg border border-gray-100 p-4 dark:border-dark-700">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.enabled') }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.enabledHint') }}</p>
                </div>
                <Toggle v-model="settingsForm.enabled" />
              </div>

              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">{{ t('admin.oauthSleeper.defaultThreshold') }}</span>
                  <input
                    v-model.number="settingsForm.threshold_percent"
                    type="number"
                    min="1"
                    max="100"
                    step="0.1"
                    class="input"
                  />
                  <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.thresholdHint') }}</span>
                </label>

                <label class="block">
                  <span class="input-label">{{ t('admin.oauthSleeper.scanInterval') }}</span>
                  <input
                    v-model.number="settingsForm.scan_interval_seconds"
                    type="number"
                    min="30"
                    max="86400"
                    step="1"
                    class="input"
                  />
                  <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.intervalHint') }}</span>
                </label>

                <label class="block">
                  <span class="input-label">{{ t('admin.oauthSleeper.maxSleepPerScan') }}</span>
                  <input
                    v-model.number="settingsForm.max_sleep_per_scan"
                    type="number"
                    min="1"
                    max="100"
                    step="1"
                    class="input"
                  />
                  <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.maxSleepHint') }}</span>
                </label>

                <div class="space-y-3">
                  <span class="input-label">{{ t('admin.oauthSleeper.platformScope') }}</span>
                  <label class="flex items-center justify-between rounded-lg border border-gray-100 px-3 py-2 dark:border-dark-700">
                    <span class="text-sm text-gray-700 dark:text-gray-200">OpenAI</span>
                    <Toggle v-model="settingsForm.include_openai" />
                  </label>
                  <label class="flex items-center justify-between rounded-lg border border-gray-100 px-3 py-2 dark:border-dark-700">
                    <span class="text-sm text-gray-700 dark:text-gray-200">Anthropic</span>
                    <Toggle v-model="settingsForm.include_anthropic" />
                  </label>
                </div>
              </div>

              <div>
                <GroupSelector
                  v-model="settingsForm.group_ids"
                  :groups="oauthSleeperGroups"
                  searchable
                />
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.groupScopeHint') }}</span>
              </div>

              <div v-if="selectedThresholdGroups.length > 0" class="rounded-lg border border-gray-100 p-4 dark:border-dark-700">
                <div class="mb-3">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.groupThresholds') }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.groupThresholdsHint') }}</p>
                </div>
                <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <label
                    v-for="group in selectedThresholdGroups"
                    :key="group.id"
                    class="flex items-center gap-3 rounded border border-gray-100 px-3 py-2 dark:border-dark-700"
                  >
                    <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-200">{{ group.name }}</span>
                    <input
                      :value="settingsForm.group_threshold_percent[group.id] ?? ''"
                      type="number"
                      min="1"
                      max="100"
                      step="0.1"
                      :placeholder="formatPercent(settingsForm.threshold_percent)"
                      class="input w-24"
                      @input="updateGroupThreshold(group.id, ($event.target as HTMLInputElement).value)"
                    />
                  </label>
                </div>
              </div>

              <div class="flex justify-end gap-2 border-t border-gray-100 pt-5 dark:border-dark-700">
                <button type="button" class="btn btn-secondary" :disabled="saving" @click="resetForm">
                  {{ t('common.reset') }}
                </button>
                <button type="submit" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving">
                  <Icon name="check" size="sm" />
                  {{ saving ? t('common.saving') : t('admin.oauthSleeper.saveSettings') }}
                </button>
              </div>
            </form>
          </div>

          <div class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-6 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.sleepingAccounts') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.sleepingAccountsHint') }}</p>
              </div>
              <span class="inline-flex w-fit items-center rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                {{ t('admin.oauthSleeper.sleepingCount', { count: status?.sleeping_accounts?.length ?? 0 }) }}
              </span>
            </div>

            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.account') }}</th>
                    <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.platform') }}</th>
                    <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.resetAt') }}</th>
                    <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.remaining') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-800">
                  <tr v-if="!status?.sleeping_accounts?.length">
                    <td colspan="4" class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
                      {{ t('admin.oauthSleeper.noSleepingAccounts') }}
                    </td>
                  </tr>
                  <tr v-for="account in status?.sleeping_accounts ?? []" :key="account.account_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/60">
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-900 dark:text-white">
                      <div class="font-medium">{{ account.account_name || t('admin.oauthSleeper.unnamedAccount') }}</div>
                      <div class="text-xs text-gray-400">#{{ account.account_id }}</div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <span class="inline-flex rounded-md border px-2 py-1 text-xs font-medium" :class="platformBadgeClass(account.platform)">
                        {{ platformLabel(account.platform) }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(account.rate_limit_reset_at) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ formatRemaining(account.remaining_seconds) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div v-if="lastScanResult" class="rounded-lg border border-primary-200 bg-primary-50 px-4 py-3 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-200">
          {{ t('admin.oauthSleeper.scanResult', { scanned: lastScanResult.scanned, triggered: lastScanResult.triggered }) }}
        </div>

        <div class="card">
          <div class="flex flex-col gap-4 border-b border-gray-100 px-6 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.oauthSleeper.eventsTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.eventsHint') }}</p>
            </div>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="eventsLoading" @click="loadEvents">
              <Icon name="refresh" size="sm" :class="eventsLoading ? 'animate-spin' : ''" />
              {{ t('common.refresh') }}
            </button>
          </div>

          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.time') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.account') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.platform') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.window') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.utilization') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.resetAt') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.table.previousReset') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-800">
                <tr v-if="eventsLoading">
                  <td colspan="7" class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
                </tr>
                <tr v-else-if="events.length === 0">
                  <td colspan="7" class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.oauthSleeper.noEvents') }}</td>
                </tr>
                <template v-else>
                  <tr v-for="event in events" :key="event.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/60">
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(event.created_at) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-900 dark:text-white">
                      <div class="font-medium">{{ event.account_name || t('admin.oauthSleeper.unnamedAccount') }}</div>
                      <div class="text-xs text-gray-400">#{{ event.account_id }}</div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <span class="inline-flex rounded-md border px-2 py-1 text-xs font-medium" :class="platformBadgeClass(event.platform)">
                        {{ platformLabel(event.platform) }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ windowLabel(event.window) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">
                      <div class="font-medium">{{ formatPercent(event.utilization_percent) }}</div>
                      <div class="text-xs text-gray-400">{{ t('admin.oauthSleeper.thresholdMeta', { threshold: formatPercent(event.threshold_percent) }) }}</div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(event.reset_at) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(event.previous_rate_limit_reset_at) }}</td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="onPageChange"
            @update:pageSize="onPageSizeChange"
          />
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type {
  OAuthSleeperEvent,
  OAuthSleeperScanResult,
  OAuthSleeperSettings,
  OAuthSleeperStatus,
} from '@/api/admin/oauthSleeper'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'

const { t } = useI18n()
const appStore = useAppStore()

const defaultSettings: OAuthSleeperSettings = {
  enabled: false,
  threshold_percent: 90,
  group_threshold_percent: {},
  scan_interval_seconds: 300,
  max_sleep_per_scan: 3,
  include_openai: true,
  include_anthropic: true,
  group_ids: [],
}

const loading = ref(false)
const saving = ref(false)
const scanning = ref(false)
const eventsLoading = ref(false)
const status = ref<OAuthSleeperStatus | null>(null)
const savedSettings = ref<OAuthSleeperSettings>({ ...defaultSettings })
const settingsForm = reactive<OAuthSleeperSettings>({ ...defaultSettings })
const events = ref<OAuthSleeperEvent[]>([])
const groups = ref<AdminGroup[]>([])
const lastScanResult = ref<OAuthSleeperScanResult | null>(null)
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0,
})

const overviewItems = computed(() => [
  {
    key: 'status',
    label: t('admin.oauthSleeper.overview.status'),
    value: status.value?.enabled ? t('common.enabled') : t('common.disabled'),
    badge: status.value?.enabled ? t('admin.oauthSleeper.running') : t('admin.oauthSleeper.paused'),
    badgeClass: status.value?.enabled
      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
      : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
    meta: groupScopeText.value,
    icon: 'shield' as const,
    iconClass: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300',
  },
  {
    key: 'threshold',
    label: t('admin.oauthSleeper.overview.threshold'),
    value: formatPercent(status.value?.threshold_percent ?? settingsForm.threshold_percent),
    badge: isAccelerated.value ? t('admin.oauthSleeper.accelerated') : undefined,
    badgeClass: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300',
    meta: effectiveIntervalText.value,
    icon: 'chart' as const,
    iconClass: 'bg-sky-50 text-sky-600 dark:bg-sky-900/20 dark:text-sky-300',
  },
  {
    key: 'lastScan',
    label: t('admin.oauthSleeper.overview.lastScan'),
    value: formatDateTime(status.value?.last_scan_at),
    meta: t('admin.oauthSleeper.lastScanMeta', {
      scanned: status.value?.last_scanned ?? 0,
      triggered: status.value?.last_triggered ?? 0,
    }),
    icon: 'clock' as const,
    iconClass: 'bg-violet-50 text-violet-600 dark:bg-violet-900/20 dark:text-violet-300',
  },
  {
    key: 'sleeping',
    label: t('admin.oauthSleeper.overview.sleeping'),
    value: String(status.value?.sleeping_accounts?.length ?? 0),
    meta: t('admin.oauthSleeper.maxPerGroupMeta', { count: status.value?.max_sleep_per_scan ?? settingsForm.max_sleep_per_scan }),
    icon: 'ban' as const,
    iconClass: 'bg-amber-50 text-amber-600 dark:bg-amber-900/20 dark:text-amber-300',
  },
])

const oauthSleeperGroups = computed(() =>
  groups.value.filter((group) => {
    if (group.platform === 'openai') return settingsForm.include_openai
    if (group.platform === 'anthropic') return settingsForm.include_anthropic
    return false
  })
)

const selectedThresholdGroups = computed(() => {
  const selected = new Set(settingsForm.group_ids)
  return oauthSleeperGroups.value.filter((group) => selected.has(group.id))
})

const selectedGroupNames = computed(() => {
  const namesByID = new Map(groups.value.map((group) => [group.id, group.name]))
  return (status.value?.group_ids ?? settingsForm.group_ids)
    .map((groupID) => namesByID.get(groupID))
    .filter((name): name is string => Boolean(name))
})

const groupScopeText = computed(() => {
  const names = selectedGroupNames.value
  if (names.length === 0) return t('admin.oauthSleeper.noGroupSelected')
  if (names.length <= 2) return names.join(' / ')
  return t('admin.oauthSleeper.groupScopeMeta', { count: names.length, names: names.slice(0, 2).join(' / ') })
})

const isAccelerated = computed(() =>
  Boolean(status.value?.accelerated_until && (status.value?.accelerated_group_ids?.length ?? 0) > 0)
)

const acceleratedGroupNames = computed(() => {
  const namesByID = new Map(groups.value.map((group) => [group.id, group.name]))
  return (status.value?.accelerated_group_ids ?? []).map((groupID) => namesByID.get(groupID) ?? `#${groupID}`)
})

const acceleratedScopeText = computed(() => {
  const names = acceleratedGroupNames.value
  if (names.length === 0) return t('admin.oauthSleeper.noGroupSelected')
  if (names.length <= 2) return names.join(' / ')
  return t('admin.oauthSleeper.acceleratedGroupScopeMeta', { count: names.length, names: names.slice(0, 2).join(' / ') })
})

const accelerationMetaText = computed(() =>
  t('admin.oauthSleeper.acceleratedUntilMeta', {
    names: acceleratedScopeText.value,
    time: formatDateTime(status.value?.accelerated_until),
  })
)

const effectiveIntervalText = computed(() => {
  const configured = status.value?.scan_interval_seconds ?? settingsForm.scan_interval_seconds
  const effective = status.value?.effective_scan_interval_seconds ?? configured
  if (effective < configured) {
    return t('admin.oauthSleeper.effectiveIntervalAcceleratedMeta', { configured, effective })
  }
  return t('admin.oauthSleeper.effectiveIntervalMeta', { configured, effective })
})

function applySettings(settings: OAuthSleeperSettings) {
  const normalized = {
    ...defaultSettings,
    ...settings,
    group_ids: [...(settings.group_ids ?? [])],
    group_threshold_percent: { ...(settings.group_threshold_percent ?? {}) },
  }
  savedSettings.value = normalized
  Object.assign(settingsForm, normalized)
}

function resetForm() {
  Object.assign(settingsForm, savedSettings.value)
}

function buildSettingsPayload(): OAuthSleeperSettings {
  const selected = new Set(settingsForm.group_ids)
  const groupThresholds: Record<number, number> = {}
  for (const [rawGroupID, rawThreshold] of Object.entries(settingsForm.group_threshold_percent ?? {})) {
    const groupID = Number(rawGroupID)
    const threshold = Number(rawThreshold)
    if (selected.has(groupID) && Number.isFinite(threshold)) {
      groupThresholds[groupID] = threshold
    }
  }
  return {
    enabled: settingsForm.enabled,
    threshold_percent: Number(settingsForm.threshold_percent),
    group_threshold_percent: groupThresholds,
    scan_interval_seconds: Number(settingsForm.scan_interval_seconds),
    max_sleep_per_scan: Number(settingsForm.max_sleep_per_scan),
    include_openai: settingsForm.include_openai,
    include_anthropic: settingsForm.include_anthropic,
    group_ids: [...settingsForm.group_ids],
  }
}

function updateGroupThreshold(groupID: number, value: string) {
  const next = { ...(settingsForm.group_threshold_percent ?? {}) }
  const trimmed = value.trim()
  if (trimmed === '') {
    delete next[groupID]
  } else {
    next[groupID] = Number(trimmed)
  }
  settingsForm.group_threshold_percent = next
}

async function loadGroups() {
  const [openaiGroups, anthropicGroups] = await Promise.all([
    adminAPI.groups.getAll('openai'),
    adminAPI.groups.getAll('anthropic'),
  ])
  groups.value = [...openaiGroups, ...anthropicGroups]
}

async function loadStatus() {
  status.value = await adminAPI.oauthSleeper.getStatus()
}

async function loadSettings() {
  const settings = await adminAPI.oauthSleeper.getSettings()
  applySettings(settings)
}

async function loadInitial() {
  loading.value = true
  try {
    await Promise.all([loadGroups(), loadSettings(), loadStatus(), loadEvents()])
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.oauthSleeper.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  try {
    await Promise.all([loadStatus(), loadEvents()])
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.oauthSleeper.loadFailed')))
  }
}

async function saveSettings() {
  const payload = buildSettingsPayload()
  if (!payload.include_openai && !payload.include_anthropic) {
    appStore.showError(t('admin.oauthSleeper.platformRequired'))
    return
  }
  if (payload.enabled && payload.group_ids.length === 0) {
    appStore.showError(t('admin.oauthSleeper.groupRequired'))
    return
  }

  saving.value = true
  try {
    const updated = await adminAPI.oauthSleeper.updateSettings(payload)
    applySettings(updated)
    await loadStatus()
    appStore.showSuccess(t('admin.oauthSleeper.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.oauthSleeper.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function runScanOnce() {
  if (scanning.value) return
  scanning.value = true
  try {
    const result = await adminAPI.oauthSleeper.scanOnce()
    lastScanResult.value = result
    appStore.showSuccess(t('admin.oauthSleeper.scanSuccess', { triggered: result.triggered }))
    pagination.page = 1
    await Promise.all([loadStatus(), loadEvents()])
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.oauthSleeper.scanFailed')))
  } finally {
    scanning.value = false
  }
}

async function loadEvents() {
  eventsLoading.value = true
  try {
    const result = await adminAPI.oauthSleeper.listEvents({
      page: pagination.page,
      page_size: pagination.page_size,
    })
    events.value = result.items ?? []
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
    pagination.pages = result.pages
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.oauthSleeper.eventsFailed')))
  } finally {
    eventsLoading.value = false
  }
}

function onPageChange(page: number) {
  pagination.page = page
  void loadEvents()
}

function onPageSizeChange(pageSize: number) {
  pagination.page = 1
  pagination.page_size = pageSize
  void loadEvents()
}

function formatDateTime(value: string | null | undefined): string {
  return formatDateTimeValue(value) || '-'
}

function formatPercent(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-'
  return `${value.toFixed(value % 1 === 0 ? 0 : 1)}%`
}

function formatRemaining(seconds: number | null | undefined): string {
  if (!seconds || seconds <= 0) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return t('admin.oauthSleeper.durationDaysHours', { days, hours })
  if (hours > 0) return t('admin.oauthSleeper.durationHoursMinutes', { hours, minutes })
  return t('admin.oauthSleeper.durationMinutes', { minutes: Math.max(minutes, 1) })
}

function windowLabel(window: string): string {
  const key = `admin.oauthSleeper.windows.${window}`
  const translated = t(key)
  return translated === key ? window : translated
}

onMounted(loadInitial)
</script>
