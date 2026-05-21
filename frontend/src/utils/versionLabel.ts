const SUMIRE_VERSION_SUFFIX = 'Sumire'

export function formatSumireVersionLabel(version: string | null | undefined): string {
  const trimmed = (version ?? '').trim()
  if (!trimmed) return ''
  if (trimmed.endsWith(` - ${SUMIRE_VERSION_SUFFIX}`)) return trimmed

  const prefixed = trimmed.startsWith('v') ? trimmed : `v${trimmed}`
  return `${prefixed} - ${SUMIRE_VERSION_SUFFIX}`
}
