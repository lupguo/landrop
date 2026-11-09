import { expect, test } from 'vitest'
import { looksLikeCode } from './detect'

test('looksLikeCode', () => {
  expect(looksLikeCode('kubectl get pods -n prod | grep Error')).toBe(true)
  expect(looksLikeCode('func a() {\n  return 1\n}')).toBe(true)
  expect(looksLikeCode('see you at 5pm')).toBe(false)
  expect(looksLikeCode('Line one of a note.\nLine two of a note.')).toBe(false)
})

test('looksLikeCode extra cases', () => {
  expect(looksLikeCode('$ ls -la')).toBe(true)
  expect(looksLikeCode('git status')).toBe(true)
  expect(looksLikeCode('docker ps')).toBe(true)
  expect(looksLikeCode('PS C:\\> dir\nfoo')).toBe(true)
  expect(looksLikeCode('a\n\tindented\n    also')).toBe(true)
  expect(looksLikeCode('x = 1;\ny = 2')).toBe(true)
  expect(looksLikeCode('https://grafana.internal/d/x')).toBe(false)
  expect(looksLikeCode('')).toBe(false)
  expect(looksLikeCode('Shopping list\nmilk and eggs')).toBe(false)
})
