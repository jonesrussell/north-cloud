import { test, expect } from '@playwright/test'

test('unauthenticated user is redirected to login', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await expect(page.locator('h1')).toHaveText('North Cloud')
})

test('login page has username and password fields', async ({ page }) => {
  await page.goto('/login')
  await expect(page.locator('input[type="text"]')).toBeVisible()
  await expect(page.locator('input[type="password"]')).toBeVisible()
  await expect(page.locator('button[type="submit"]')).toHaveText('Sign In')
})

test('login form shows error on invalid credentials', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[type="text"]', 'baduser')
  await page.fill('input[type="password"]', 'badpass')
  await page.click('button[type="submit"]')
  // Auth service likely not running — expect error message
  await expect(page.getByText('Invalid credentials')).toBeVisible({ timeout: 5000 })
})

test('404 page shows for unknown routes', async ({ page }) => {
  // Set a fake token to bypass auth guard
  await page.goto('/login')
  await page.evaluate(() => {
    const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
    localStorage.setItem('dashboard_token', `header.${payload}.signature`)
  })
  await page.goto('/nonexistent-page')
  await expect(page.getByText('404')).toBeVisible()
  await expect(page.getByText('Back to dashboard')).toBeVisible()
})

test('sidebar navigation renders when authenticated', async ({ page }) => {
  await page.goto('/login')
  await page.evaluate(() => {
    const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
    localStorage.setItem('dashboard_token', `header.${payload}.signature`)
  })
  await page.goto('/')
  await expect(page.getByText('North Cloud')).toBeVisible()
  await expect(page.getByText('Pipeline Overview')).toBeVisible()
  await expect(page.getByText('Sources')).toBeVisible()
  await expect(page.getByText('Crawl Jobs')).toBeVisible()
})
