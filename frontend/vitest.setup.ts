/**
 * Vitest runs in a plain Node environment where there is no DOM, but several
 * modules under test (i18n language persistence) touch `localStorage`.
 *
 * This stub is deliberately minimal: it exists so the module imports and
 * `setLanguage` calls do not throw, not to emulate a browser. Component-level
 * behaviour (direction, RTL layout) is covered by the app itself, not here.
 */
class MemoryStorage {
  private store = new Map<string, string>()

  getItem(key: string): string | null {
    return this.store.has(key) ? (this.store.get(key) as string) : null
  }

  setItem(key: string, value: string): void {
    this.store.set(key, String(value))
  }

  removeItem(key: string): void {
    this.store.delete(key)
  }

  clear(): void {
    this.store.clear()
  }

  key(index: number): string | null {
    return Array.from(this.store.keys())[index] ?? null
  }

  get length(): number {
    return this.store.size
  }
}

if (typeof globalThis.localStorage === 'undefined') {
  ;(globalThis as { localStorage?: Storage }).localStorage = new MemoryStorage() as unknown as Storage
}
