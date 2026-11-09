import { expect, test } from 'vitest'
import { LANGS, dicts, setLang, t } from './index'
import { en } from './en'

test('i18n parity', () => {
  for (const l of LANGS) for (const k of Object.keys(en)) expect(dicts[l][k], `${l}.${k}`).toBeTruthy()
})
test('t params + fallback', () => {
  setLang('en')
  expect(t('feed.expand', { n: 14 })).toBe('Show all (14 lines)')
})
