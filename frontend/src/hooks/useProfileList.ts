import { useCallback, useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { ConnectionStatus, Profile } from '../types'

/**
 * useProfileList — the load + live-update plumbing shared by the four category
 * pages (V2Ray, OpenVPN, Warp/WireGuard, Aether). Each page used to carry its
 * own copy of:
 *
 *   - loadProfiles()   (with slightly different Array.isArray guards — the
 *                      Warp page's missing guard blanked the whole list once)
 *   - a status:changed / category-specific update listener that keeps the
 *     in-memory list in sync with the store
 *
 * The connect/disconnect/delete actions stay in the pages: their UX differs
 * (optimistic status, confirmation dialog, onStatusChange reporting), and the
 * hook should not pretend those are identical.
 *
 * @param category   the EffectiveCategory filter, empty for all profiles
 * @param refreshKey an optional external dependency that forces a reload
 *                   (e.g. a modal-open flag) while staying null-safe
 */
export function useProfileList(category: string) {
  const [profiles, setProfiles] = useState<Profile[]>([])

  const loadProfiles = useCallback(async () => {
    try {
      const list = category
        ? await api.getProfilesByCategory(category)
        : await api.getProfiles()
      // The guard placement is the point: some backends legitimately return
      // null on an empty category, and a bare setProfiles(null) blanks the
      // entire page.
      if (list && Array.isArray(list)) setProfiles(list)
    } catch (err) {
      console.error(`Failed to load profiles (category=${category || 'all'}):`, err)
    }
  }, [category])

  useEffect(() => {
    void loadProfiles()
  }, [loadProfiles])

  // Keep an open list in sync with backend-side mutations (batch ping
  // completion, subscription refresh, failover bookkeeping) without each page
  // re-implementing the event subscription.
  useEffect(() => {
    const events = ['profiles:ping_complete']
    const unsubs = events.map(name =>
      api.onEvent(name, (updated: Profile[] | unknown) => {
        if (!updated || !Array.isArray(updated)) return
        if (!category) {
          setProfiles(updated)
          return
        }
        const list = updated as Profile[]
        if (list.some(p => (p as { category?: string }).category === category)) {
          setProfiles([...list].filter(p => (p as { category?: string }).category === category))
        }
      })
    )
    return () => unsubs.forEach(u => u())
  }, [category])

  const applyStatusUpdate = useCallback(
    (list: Profile[], status: Pick<ConnectionStatus, 'state' | 'profile_id'>): Profile[] =>
      list.map(p => ({
        ...p,
        is_connected:
          (status.state === 'connected' || status.state === 'connecting') &&
          status.profile_id === p.id,
      })),
    []
  )

  const markConnected = useCallback(
    (id: string, connected: boolean) => {
      setProfiles(prev =>
        prev.map(p => (p.id === id ? { ...p, is_connected: connected } : p))
      )
    },
    []
  )

  return { profiles, setProfiles, loadProfiles, markConnected, applyStatusUpdate }
}

// Keep the ConnectionStatus import meaningful for consumers re-exporting types.
export type { ConnectionStatus }
