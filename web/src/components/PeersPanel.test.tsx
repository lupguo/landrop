// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/preact'
import { afterEach, expect, test } from 'vitest'
import { setLang, t } from '../i18n'
import { me, peers } from '../live'
import { PeersPanel } from './PeersPanel'

afterEach(cleanup)

const base = { no_auth: false, public_url: '', mdns_url: '', code: '', join_url: '', has_dir: false, max_size: 1 }
const UA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/537.36 Chrome/120.0 Safari/537.36'

function setup(role: 'host' | 'peer') {
  setLang('en')
  me.value = { ...base, peer_id: 'p1', role }
  peers.value = [
    { peer_id: 'p1', name: 'Me', ua: UA, ip: '127.0.0.1', is_host: role === 'host' },
    { peer_id: 'p2', name: 'Phone', ua: UA, ip: '192.168.1.5', is_host: false },
  ]
}

test('PeersPanel hides kick for peer role', () => {
  setup('peer')
  render(<PeersPanel onClose={() => {}} />)
  expect(screen.queryAllByText(t('peers.kick')).length).toBe(0)
  expect(screen.getByText('192.168.1.5')).toBeTruthy()
})

test('PeersPanel host: kick only for non-self, badges shown', () => {
  setup('host')
  render(<PeersPanel onClose={() => {}} />)
  expect(screen.getAllByText(t('peers.kick')).length).toBe(1)
  expect(screen.getByText(t('peers.this_device'))).toBeTruthy()
  expect(screen.getByText(t('peers.host'))).toBeTruthy()
  expect(screen.getAllByText('Mac · Chrome').length).toBe(2)
})

test('PeersPanel hides kick in no-auth mode', () => {
  setup('host')
  me.value = { ...me.value!, no_auth: true }
  render(<PeersPanel onClose={() => {}} />)
  expect(screen.queryAllByText(t('peers.kick')).length).toBe(0)
})

test('PeersPanel renders a localized name for empty peer names', () => {
  setup('peer')
  peers.value = [{ peer_id: 'p3', name: '', ua: UA, ip: '192.168.1.9', is_host: false }]
  render(<PeersPanel onClose={() => {}} />)
  expect(screen.getByText(t('feed.unknown'))).toBeTruthy()
})
