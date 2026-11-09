import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'
import preact from '@preact/preset-vite'
import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

// emptyOutDir wipes the committed .gitkeep that the Go embed needs; put it back.
function keepGitkeep(): Plugin {
  let out = ''
  return {
    name: 'landrop-gitkeep',
    apply: 'build',
    configResolved(c) { out = resolve(c.root, c.build.outDir) },
    closeBundle() { writeFileSync(resolve(out, '.gitkeep'), '') },
  }
}

const target = 'http://127.0.0.1:8848'

export default defineConfig({
  plugins: [preact(), keepGitkeep()],
  build: { outDir: '../internal/webui/dist', emptyOutDir: true },
  test: { exclude: ['e2e/**', 'node_modules/**'] },
  server: { proxy: { '/api': target, '/events': target, '^/s/': target } },
})
