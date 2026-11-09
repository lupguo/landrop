import { t } from '../i18n'
import { connected } from '../live'

export function ConnBanner() {
  if (connected.value) return null
  return <div class="strip err" role="alert">{t('conn.lost')}</div>
}
