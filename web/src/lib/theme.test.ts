// @vitest-environment jsdom
import { expect, test } from 'vitest'
import { applyTheme, setTheme, theme } from './theme'
import { normalizeDeviceName } from './device'

test('theme applies data-theme; system removes it', () => {
  setTheme('dark')
  expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  expect(localStorage.getItem('landrop.theme')).toBe('dark')
  setTheme('system')
  expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
  setTheme('light')
  document.documentElement.removeAttribute('data-theme')
  applyTheme()
  expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  expect(theme.value).toBe('light')
})

test('normalizeDeviceName', () => {
  expect(normalizeDeviceName('  Bob  ', 'Mac · Chrome')).toBe('Bob')
  expect(normalizeDeviceName('   ', 'Mac · Chrome')).toBe('Mac · Chrome')
  expect(normalizeDeviceName('x'.repeat(60), 'g')).toBe('x'.repeat(40))
})
