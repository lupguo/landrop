import { deviceName, NAME_MAX, saveDeviceName } from '../lib/device'
import { t } from '../i18n'
import { setTheme, theme, type Theme } from '../lib/theme'
import { Dialog } from './Dialog'
import { LangSwitch } from './LangSwitch'

const THEMES: Theme[] = ['system', 'light', 'dark']

export function Settings(props: { onClose: () => void }) {
  return (
    <Dialog title={t('settings.title')} onClose={props.onClose}>
      <div class="field">
        <label for="set-name">{t('settings.name')}</label>
        <input
          id="set-name"
          class="inp"
          type="text"
          maxLength={NAME_MAX}
          value={deviceName.value}
          onChange={(e) => {
            const el = e.target as HTMLInputElement
            el.value = saveDeviceName(el.value)
          }}
        />
        <p class="hint">{t('settings.name_hint')}</p>
      </div>
      <div class="field">
        <span class="lbl" id="set-theme">{t('settings.theme')}</span>
        <div class="langsw" role="group" aria-labelledby="set-theme">
          {THEMES.map((th) => (
            <button key={th} class={`btn${theme.value === th ? ' on' : ''}`} aria-pressed={theme.value === th} onClick={() => setTheme(th)}>
              {t(`settings.theme_${th}`)}
            </button>
          ))}
        </div>
      </div>
      <div class="field">
        <span class="lbl">{t('settings.language')}</span>
        <LangSwitch />
      </div>
      <p class="hint">{t('settings.autosave_note')}</p>
    </Dialog>
  )
}
