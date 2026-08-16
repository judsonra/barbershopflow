import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// @testing-library/react only auto-registers its afterEach(cleanup) when it
// detects a *global* afterEach — which requires vitest's `globals: true`.
// This project keeps `globals: false` (explicit imports, matching the
// existing *.test.ts files), so cleanup is wired up here instead.
afterEach(cleanup)
