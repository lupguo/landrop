import { expect, test } from 'vitest'
import { avatarColor, avatarLetter, AVATAR_HUES, hhmm, lineCount, utf8Len, extLabel, isImageMime } from './meta'

test('avatarColor is stable and from the 8 hues', () => {
  expect(AVATAR_HUES.length).toBe(8)
  expect(avatarColor('peer-1')).toBe(avatarColor('peer-1'))
  expect(AVATAR_HUES).toContain(avatarColor('peer-1'))
  expect(AVATAR_HUES).toContain(avatarColor(''))
  const set = new Set(['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'].map(avatarColor))
  expect(set.size).toBeGreaterThan(1)
})

test('avatarLetter', () => {
  expect(avatarLetter('macBook')).toBe('M')
  expect(avatarLetter('  win')).toBe('W')
  expect(avatarLetter('')).toBe('?')
  expect(avatarLetter('😀x')).toBe('😀')
  expect(avatarLetter('张三')).toBe('张')
})

test('hhmm local', () => {
  const d = new Date(2026, 10, 9, 7, 5)
  expect(hhmm(d.toISOString())).toBe('07:05')
  expect(hhmm('nope')).toBe('')
})

test('lineCount / utf8Len', () => {
  expect(lineCount('a')).toBe(1)
  expect(lineCount('a\nb\r\nc')).toBe(3)
  expect(utf8Len('中')).toBe(3)
  expect(utf8Len('ab')).toBe(2)
})

test('extLabel / isImageMime', () => {
  expect(extLabel('app-error.log')).toBe('LOG')
  expect(extLabel('x.jpeg2000')).toBe('JPEG')
  expect(extLabel('Makefile')).toBe('')
  expect(extLabel('.bashrc')).toBe('')
  expect(isImageMime('image/png')).toBe(true)
  expect(isImageMime('image/svg+xml')).toBe(false)
})
