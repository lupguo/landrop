import { ApiError } from '../api'
import { t } from '../i18n'

const KNOWN = ['too_large', 'disk_full', 'empty', 'no_file', 'bad_origin', 'unauthorized', 'network', 'aborted', 'too_many', 'bad_attachment'] as const
type Known = (typeof KNOWN)[number]

/** Localized message for an API error code (or an ApiError / unknown error). */
export function errText(e: unknown): string {
  const code = e instanceof ApiError ? e.code : typeof e === 'string' ? e : ''
  if ((KNOWN as readonly string[]).includes(code)) return t(`err.${code as Known}`)
  return t('err.generic')
}
