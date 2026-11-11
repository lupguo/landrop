// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/preact'
import { afterEach, expect, test, vi } from 'vitest'
import { setLang, t } from '../i18n'
import { me } from '../live'
import { InvitePanel } from './InvitePanel'

afterEach(cleanup)

const base = {
  peer_id: 'p1', role: 'peer' as const, no_auth: false, public_url: 'http://21.6.149.203:9959',
  mdns_url: 'http://drop.local:9959', has_dir: false, max_size: 1,
  code: '4821', join_url: 'http://21.6.149.203:9959/?t=abc',
}

test('QR is the token link; copyable address is the plain one; code is shown', () => {
  setLang('en')
  me.value = { ...base }
  render(<InvitePanel onClose={() => {}} />)
  const img = screen.getByAltText(t('invite.qr_alt')) as HTMLImageElement
  expect(img.getAttribute('src')).toBe('/api/qr?data=' + encodeURIComponent('http://21.6.149.203:9959/?t=abc'))
  expect((screen.getByLabelText(t('invite.address')) as HTMLInputElement).value).toBe('http://21.6.149.203:9959')
  expect(screen.getByText('4821')).toBeTruthy()
  expect(screen.getByText(t('invite.scan'))).toBeTruthy()
  // only the address has a copy button
  expect(screen.getAllByText(t('item.copy')).length).toBe(1)
})

test('no-auth: no code, QR is the plain address', () => {
  setLang('en')
  me.value = { ...base, no_auth: true, code: '', join_url: 'http://21.6.149.203:9959' }
  render(<InvitePanel onClose={() => {}} />)
  expect(screen.queryByText(t('invite.code'))).toBeNull()
  expect(screen.queryByText(t('invite.scan'))).toBeNull()
  expect(screen.getByAltText(t('invite.qr_alt')).getAttribute('src')).toBe('/api/qr?data=' + encodeURIComponent('http://21.6.149.203:9959'))
})

test('Esc and outside click close; inside click does not', () => {
  setLang('en')
  me.value = { ...base }
  const onClose = vi.fn()
  render(<InvitePanel onClose={onClose} />)
  fireEvent.mouseDown(screen.getByText('4821'))
  expect(onClose).not.toHaveBeenCalled()
  fireEvent.mouseDown(document.body)
  expect(onClose).toHaveBeenCalledTimes(1)
  fireEvent.keyDown(document, { key: 'Escape' })
  expect(onClose).toHaveBeenCalledTimes(2)
})
