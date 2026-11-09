import { signal } from '@preact/signals'
import { useEffect } from 'preact/hooks'

/** Number of dialogs/panels currently mounted; global shortcuts (paste) stand down while > 0. */
export const openOverlay = signal(0)

export function useOverlay(): void {
  useEffect(() => {
    openOverlay.value++
    return () => { openOverlay.value = Math.max(0, openOverlay.value - 1) }
  }, [])
}
