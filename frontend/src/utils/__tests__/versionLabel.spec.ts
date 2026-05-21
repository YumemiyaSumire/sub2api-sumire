import { describe, expect, it } from 'vitest'

import { formatSumireVersionLabel } from '../versionLabel'

describe('formatSumireVersionLabel', () => {
  it('formats plain semver with v prefix and Sumire suffix', () => {
    expect(formatSumireVersionLabel('0.1.129')).toBe('v0.1.129 - Sumire')
  })

  it('does not duplicate an existing v prefix or Sumire suffix', () => {
    expect(formatSumireVersionLabel('v0.1.129')).toBe('v0.1.129 - Sumire')
    expect(formatSumireVersionLabel('v0.1.129 - Sumire')).toBe('v0.1.129 - Sumire')
  })

  it('returns empty string for missing versions', () => {
    expect(formatSumireVersionLabel('')).toBe('')
    expect(formatSumireVersionLabel(undefined)).toBe('')
  })
})
