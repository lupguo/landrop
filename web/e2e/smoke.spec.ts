import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'

const BASE = process.env.LANDROP_BASE ?? 'http://127.0.0.1:18848'
const LOG = process.env.LANDROP_LOG ?? 'test-results/server.log'

function accessCode(): string {
  const m = /Access code: (\d{4})/.exec(readFileSync(LOG, 'utf8'))
  if (!m) throw new Error(`no access code found in ${LOG}`)
  return m[1]
}

async function newDevice(browser: Browser, clipboard = false): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext({
    baseURL: BASE,
    permissions: clipboard ? ['clipboard-read', 'clipboard-write'] : [],
    acceptDownloads: true,
  })
  const page = await ctx.newPage()
  await page.goto(BASE + '/')
  await page.getByLabel('Access code').fill(accessCode())
  await expect(page.getByLabel('Message')).toBeVisible()
  return { ctx, page }
}

async function send(page: Page, text: string) {
  await page.getByLabel('Message').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

test('login by code shows the composer', async ({ browser }) => {
  const { ctx, page } = await newDevice(browser)
  await expect(page.getByLabel('Message')).toBeVisible()
  await ctx.close()
})

test('text sent in one context appears in another and copies', async ({ browser }) => {
  const a = await newDevice(browser)
  const b = await newDevice(browser, true)
  const text = 'kubectl get pods'
  await send(a.page, text)
  const card = b.page.getByTestId('item').filter({ hasText: text }).first()
  await expect(card).toBeVisible({ timeout: 1000 })
  await card.getByRole('button', { name: 'Copy' }).click()
  await expect(b.page.getByRole('status').getByText('Copied')).toBeVisible()
  expect(await b.page.evaluate(() => navigator.clipboard.readText())).toBe(text)
  await a.ctx.close()
  await b.ctx.close()
})

test('long text collapses and expands', async ({ browser }) => {
  const { ctx, page } = await newDevice(browser)
  const lines = Array.from({ length: 14 }, (_, i) => `collapse-line-${i + 1}`)
  await send(page, lines.join('\n'))
  const card = page.getByTestId('item').filter({ hasText: 'collapse-line-1' }).first()
  await card.getByRole('button', { name: 'Show all (14 lines)' }).click()
  await expect(card.getByRole('button', { name: 'Collapse' })).toBeVisible()
  await ctx.close()
})

test('2MB upload shows up elsewhere and downloads under its name', async ({ browser }) => {
  const a = await newDevice(browser)
  const b = await newDevice(browser)
  await a.page.locator('input[type=file]').setInputFiles({
    name: 'app.log',
    mimeType: 'text/plain',
    buffer: Buffer.alloc(2 * 1024 * 1024, 'x'),
  })
  await a.page.getByRole('button', { name: 'Send', exact: true }).click()
  const card = b.page.getByTestId('item').filter({ hasText: 'app.log' }).first()
  await expect(card).toBeVisible({ timeout: 10_000 })
  const [dl] = await Promise.all([
    b.page.waitForEvent('download'),
    card.getByRole('link', { name: 'Download' }).click(),
  ])
  expect(dl.suggestedFilename()).toBe('app.log')
  await a.ctx.close()
  await b.ctx.close()
})

test('share link works once, then is gone', async ({ browser }) => {
  const a = await newDevice(browser)
  await a.page.locator('input[type=file]').setInputFiles({
    name: 'once.bin',
    mimeType: 'application/octet-stream',
    buffer: Buffer.alloc(64 * 1024, 'y'),
  })
  await a.page.getByRole('button', { name: 'Send', exact: true }).click()
  const card = a.page.getByTestId('item').filter({ hasText: 'once.bin' }).first()
  await expect(card).toBeVisible({ timeout: 10_000 })
  await card.getByRole('button', { name: 'Share' }).click()
  await a.page.getByRole('button', { name: 'Create link' }).click()
  const url = await a.page.locator('#sh-url').inputValue()
  // The displayed URL uses the LAN address; reach the same server via the test base.
  const local = BASE + new URL(url).pathname + new URL(url).search

  const guest = await browser.newContext({ acceptDownloads: true })
  const gp = await guest.newPage()
  await gp.goto(local)
  const [dl] = await Promise.all([
    gp.waitForEvent('download'),
    gp.getByRole('link', { name: 'Download' }).click(),
  ])
  expect(dl.suggestedFilename()).toBe('once.bin')
  await dl.path() // wait until fully received
  await gp.goto(local)
  await expect(gp.getByText('This link has expired or was already used.')).toBeVisible()
  await guest.close()
  await a.ctx.close()
})

const PNG_B64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='

async function pasteImages(page: Page, count: number) {
  await page.getByLabel('Message').focus()
  await page.evaluate(
    ({ b64, n }) => {
      const bin = atob(b64)
      const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
      const dt = new DataTransfer()
      for (let i = 0; i < n; i++) dt.items.add(new File([bytes], 'image.png', { type: 'image/png' }))
      const ta = document.querySelector('textarea')!
      ta.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }))
    },
    { b64: PNG_B64, n: count },
  )
}

test('pasted screenshots + text arrive as one message', async ({ browser }) => {
  const a = await newDevice(browser)
  const b = await newDevice(browser)
  const marker = `look-${Date.now()}`
  await pasteImages(a.page, 2)
  await a.page.getByLabel('Message').fill(marker)
  await expect(a.page.getByRole('button', { name: /^Remove screenshot-/ })).toHaveCount(2)
  await a.page.getByRole('button', { name: /^Remove screenshot-/ }).nth(1).click()
  await expect(a.page.getByRole('button', { name: /^Remove screenshot-/ })).toHaveCount(1)
  await a.page.getByRole('button', { name: 'Send', exact: true }).click()

  const card = b.page.getByTestId('item').filter({ hasText: marker })
  await expect(card).toHaveCount(1, { timeout: 10_000 })
  await expect(card.locator('img')).toHaveCount(1)
  await card.locator('button:has(img)').click()
  const dialog = b.page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.locator('img')).toBeVisible()
  await expect(dialog).toBeFocused() // key handlers are installed once focus has moved in
  await b.page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  const [dl] = await Promise.all([
    b.page.waitForEvent('download'),
    card.getByRole('link', { name: /^Download screenshot-/ }).click(),
  ])
  expect(dl.suggestedFilename()).toMatch(/^screenshot-/)
  await a.ctx.close()
  await b.ctx.close()
})

test('message share link is reusable within expiry', async ({ browser }) => {
  const a = await newDevice(browser)
  const marker = `share-msg-${Date.now()}`
  await a.page.locator('input[type=file]').setInputFiles([
    { name: 'one.bin', mimeType: 'application/octet-stream', buffer: Buffer.alloc(1024, 'a') },
    { name: 'two.bin', mimeType: 'application/octet-stream', buffer: Buffer.alloc(1024, 'b') },
  ])
  await a.page.getByLabel('Message').fill(marker)
  await a.page.getByRole('button', { name: 'Send', exact: true }).click()
  const card = a.page.getByTestId('item').filter({ hasText: marker })
  await expect(card).toHaveCount(1, { timeout: 10_000 })
  await card.getByRole('button', { name: 'Share' }).click()
  await expect(a.page.getByLabel('Allowed downloads')).toHaveCount(0)
  await a.page.getByRole('button', { name: 'Create link' }).click()
  const url = await a.page.locator('#sh-url').inputValue()
  const local = BASE + new URL(url).pathname + new URL(url).search

  const guest = await browser.newContext({ acceptDownloads: true })
  const gp = await guest.newPage()
  for (let i = 0; i < 2; i++) {
    await gp.goto(local)
    await expect(gp.getByText(marker)).toBeVisible()
    const [dl] = await Promise.all([
      gp.waitForEvent('download'),
      gp.getByRole('link', { name: 'Download one.bin' }).click(),
    ])
    expect(dl.suggestedFilename()).toBe('one.bin')
    await dl.path()
  }
  await guest.close()
  await a.ctx.close()
})
