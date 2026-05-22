export const SUMIRE_VERSION_SUFFIX = 'Sumire'

export function formatSumireVersionNumber(version: string | null | undefined): string {
  const trimmed = (version ?? '').trim()
  if (!trimmed) return ''
  const withoutSuffix = trimmed.endsWith(` - ${SUMIRE_VERSION_SUFFIX}`)
    ? trimmed.slice(0, -` - ${SUMIRE_VERSION_SUFFIX}`.length).trim()
    : trimmed

  return withoutSuffix.startsWith('v') ? withoutSuffix : `v${withoutSuffix}`
}

export function formatSumireVersionLabel(version: string | null | undefined): string {
  const versionNumber = formatSumireVersionNumber(version)
  if (!versionNumber) return ''
  return `${versionNumber} - ${SUMIRE_VERSION_SUFFIX}`
}
