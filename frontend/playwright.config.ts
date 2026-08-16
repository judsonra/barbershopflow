import { defineConfig, devices } from '@playwright/test'

// Runs against the real `make dev` stack (frontend on :5173 proxying /api
// to the Go backend on :8080, backed by a real Postgres) — not a static
// `vite preview` build, since every flow here (login, booking,
// impersonation) needs a live backend. No `webServer` here on purpose:
// the dev stack's own startup (Postgres health check, migrations) is
// already orchestrated by compose.dev.yml, so tests assume `make dev` is
// already up rather than racing a cold docker-compose start.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
})
