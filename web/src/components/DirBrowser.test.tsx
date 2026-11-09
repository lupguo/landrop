// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/preact'
import { afterEach, expect, test, vi } from 'vitest'
import { api } from '../api'
import { setLang, t } from '../i18n'
import { DirBrowser } from './DirBrowser'

afterEach(() => { cleanup(); vi.restoreAllMocks(); setLang('en') })

test('DirBrowser folder icon label is localized', async () => {
  vi.spyOn(api, 'dir').mockResolvedValue({ path: '', entries: [{ name: 'sub', is_dir: true, size: 0, mod_time: '2026-11-09T10:00:00Z' }] } as never)
  setLang('zh')
  render(<DirBrowser />)
  expect(await screen.findByText(t('dir.folder_icon'))).toBeTruthy()
  expect(t('dir.folder_icon')).not.toBe('dir.folder_icon')
  expect(screen.queryByText('DIR')).toBeNull()
})
