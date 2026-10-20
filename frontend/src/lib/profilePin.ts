import type { Dispatch, SetStateAction } from 'react'

import { api } from './api'
import type { Profile } from '../types'

/**
 * Pinning, shared by every profile list.
 *
 * Pinning means two different things at once — the row floats to the top of its
 * list, and the profile is offered first in the tray menu — so the two must
 * never disagree. Keeping the write in one place is what guarantees that; the
 * three list pages only own the local state they already had.
 */

/**
 * Toggle a profile's pinned state.
 *
 * Optimistic on purpose: the write is a disk-backed store update, and waiting
 * for it makes the star feel broken. A failure rolls the single row back rather
 * than refetching the list, which would discard the user's sort and scroll.
 */
export async function toggleProfilePin(
  profile: Profile,
  setProfiles: Dispatch<SetStateAction<Profile[]>>
): Promise<void> {
  const next = !profile.favorite
  setProfiles(prev => prev.map(p => (p.id === profile.id ? { ...p, favorite: next } : p)))
  try {
    await api.setProfileFavorite(profile.id, next)
  } catch (err) {
    console.error('Pin error:', err)
    setProfiles(prev => prev.map(p => (p.id === profile.id ? { ...p, favorite: !next } : p)))
  }
}

/**
 * Pinned profiles lead, whichever sort is active.
 *
 * Apply this *last*, after the page's own sort: `Array.prototype.sort` is
 * stable, so sorting only on the pin preserves whatever order the chosen sort
 * mode just established within each of the two groups. Folding the pin into
 * every comparator instead would mean writing it once per sort mode and getting
 * the precedence right in each.
 */
export function pinnedFirst(a: Profile, b: Profile): number {
  return Number(b.favorite ?? false) - Number(a.favorite ?? false)
}
