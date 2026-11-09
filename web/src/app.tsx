import { useEffect, useState } from 'preact/hooks'
import { api, ApiError } from './api'
import { LoginPage } from './components/LoginPage'
import { MainPage } from './components/MainPage'
import { Toasts } from './components/Toast'
import { t } from './i18n'
import { me, startLive, stopLive } from './live'

// Stub: Task 16 implements the share page.
function SharePage(props: { id: string }) {
  return <div class="login"><div class="card login-card">{t('share.stub')} {props.id}</div></div>
}

export function App() {
  const m = location.pathname.match(/^\/s\/([^/]+)/)
  const [checking, setChecking] = useState(!m)
  const [unreachable, setUnreachable] = useState(false)

  let retry: ReturnType<typeof setTimeout> | undefined
  let delay = 500

  async function load() {
    try {
      me.value = await api.me()
      setUnreachable(false)
      startLive()
    } catch (e) {
      me.value = null
      stopLive()
      if (!(e instanceof ApiError && e.status === 401)) {
        setUnreachable(true)
        retry = setTimeout(() => void load(), delay)
        delay = Math.min(delay * 2, 3000)
      } else {
        setUnreachable(false)
      }
    } finally {
      setChecking(false)
    }
  }

  useEffect(() => {
    if (!m) void load()
    return () => clearTimeout(retry)
  }, [])

  if (m) return <SharePage id={decodeURIComponent(m[1])} />
  if (checking) return <div class="login"><p class="mut">{t('app.loading')}</p></div>
  if (unreachable) return <div class="login"><p class="mut" role="alert">{t('app.unreachable')}</p></div>
  return (
    <>
      {me.value ? <MainPage /> : <LoginPage onSuccess={() => void load()} />}
      <Toasts />
    </>
  )
}
