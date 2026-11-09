import { LANGS, lang, setLang, t, type Lang } from '../i18n'

const NAMES: Record<Lang, string> = { en: 'EN', zh: '中文', ja: '日本語', ko: '한국어' }

export function LangSwitch() {
  return (
    <div class="langsw" role="group" aria-label={t('lang.label')}>
      {LANGS.map((l) => (
        <button key={l} class={`btn${lang.value === l ? ' on' : ''}`} aria-pressed={lang.value === l} onClick={() => setLang(l)}>
          {NAMES[l]}
        </button>
      ))}
    </div>
  )
}
