import { describe, it, expect } from 'vitest'
import { cn, formatUptime, truncateHash } from './utils'

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('handles conditional classes', () => {
    expect(cn('base', false && 'hidden', 'visible')).toBe('base visible')
  })

  it('deduplicates tailwind classes', () => {
    expect(cn('text-red-500', 'text-blue-500')).toBe('text-blue-500')
  })
})

describe('formatUptime', () => {
  it('returns the uptime string', () => {
    expect(formatUptime('1h23m')).toBe('1h23m')
  })

  it('returns dash for empty', () => {
    expect(formatUptime('')).toBe('-')
  })
})

describe('truncateHash', () => {
  it('truncates long hashes', () => {
    const hash = 'abcdef1234567890abcdef1234567890'
    expect(truncateHash(hash, 8)).toBe('abcdef12...')
  })

  it('returns dash for empty', () => {
    expect(truncateHash('')).toBe('-')
  })
})
