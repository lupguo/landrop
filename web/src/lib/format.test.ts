import { expect, test } from 'vitest'
import { formatSize } from './format'

test('formatSize', () => {
  expect(formatSize(0)).toBe('0 B')
  expect(formatSize(1536)).toBe('1.5 KB')
  expect(formatSize(13002345)).toBe('12.4 MB')
  expect(formatSize(4 * 1024 ** 3)).toBe('4.0 GB')
  expect(formatSize(512)).toBe('512 B')
})
