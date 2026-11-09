/** Copy via a hidden textarea + execCommand; works on plain-HTTP LAN origins. */
function execCopy(text: string): boolean {
  const prev = document.activeElement as HTMLElement | null
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.style.cssText = 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;pointer-events:none;'
  document.body.appendChild(ta)
  ta.focus()
  ta.select()
  ta.setSelectionRange(0, text.length)
  let ok = false
  try { ok = document.execCommand('copy') } catch { ok = false }
  document.body.removeChild(ta)
  try { prev?.focus({ preventScroll: true }) } catch { /* ignore */ }
  return ok
}

/**
 * Copy text to the clipboard. The async Clipboard API only exists in secure
 * contexts (https / localhost); on http://<LAN IP> fall back to execCommand.
 */
export async function copyText(text: string): Promise<boolean> {
  if (window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch { /* permission denied etc: try the legacy path */ }
  }
  return execCopy(text)
}
