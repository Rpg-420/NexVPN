import { describe, expect, it } from 'vitest'

import { pinnedFirst } from './profilePin'
import type { Profile } from '../types'

function profile(id: string, favorite: boolean): Profile {
  return { id, name: id, favorite } as Profile
}

describe('pinnedFirst', () => {
  it('orders pinned before unpinned', () => {
    expect(pinnedFirst(profile('a', true), profile('b', false))).toBeLessThan(0)
    expect(pinnedFirst(profile('a', false), profile('b', true))).toBeGreaterThan(0)
  })

  it('treats equal pin state as equal, preserving the caller sort order', () => {
    expect(pinnedFirst(profile('a', true), profile('b', true))).toBe(0)
    expect(pinnedFirst(profile('a', false), profile('b', false))).toBe(0)
  })

  it('treats a missing favorite flag as unpinned', () => {
    expect(pinnedFirst({ id: 'a' } as Profile, profile('b', true))).toBeGreaterThan(0)
    expect(pinnedFirst(profile('a', true), { id: 'b' } as Profile)).toBeLessThan(0)
  })

  it('is stable through Array.sort with equal pins', () => {
    const list = [profile('x', false), profile('y', false), profile('z', false)]
    const sorted = [...list].sort(pinnedFirst)
    expect(sorted.map(p => p.id)).toEqual(['x', 'y', 'z'])
  })
})
