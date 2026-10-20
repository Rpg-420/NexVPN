import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    // Node is enough for the pure-logic suites in src/lib; component behaviour
    // is verified through the real app, and jsdom would only add weight here.
    environment: 'node',
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.test.ts'],
  },
})
