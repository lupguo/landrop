import { t } from '../i18n'
import { me, peers } from '../live'

function displayUrl(u: string): string {
  return u.replace(/^https?:\/\//, '').replace(/\/$/, '')
}

export function TopBar(props: { onInviteClick: () => void; onPeersClick: () => void; onSettingsClick: () => void }) {
  const m = me.value
  const url = m ? m.mdns_url || m.public_url : ''
  return (
    <header class="topbar-wrap">
      <div class="topbar">
        <span class="logo">Lan<span>Drop</span></span>
        {url && <span class="host mono" title={url}>{displayUrl(url)}</span>}
        <span class="sp" />
        {m && <button class="btn invite-btn" onClick={props.onInviteClick} aria-haspopup="dialog">{t('topbar.invite')}</button>}
        <button class="pill" onClick={props.onPeersClick} aria-label={t('topbar.peers')}>
          <span class="dot" aria-hidden="true">●</span> {t('topbar.online', { n: peers.value.length })}
        </button>
        <button class="btn" onClick={props.onSettingsClick}>{t('topbar.settings')}</button>
      </div>
      {m?.no_auth && <div class="strip warn" role="alert">{t('topbar.no_auth')}</div>}
    </header>
  )
}
