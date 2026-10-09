import { defineConfig } from "@playwright/test"
import { readFileSync } from "node:fs"
import { join } from "node:path"

const fixtures = JSON.parse(readFileSync(join(__dirname, "..", ".run", "fixtures.json"), "utf8"))

export default defineConfig({
  testDir: "./tests",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  // Concurrent runs (several workstreams at once) must not share output dirs.
  outputDir: process.env.PW_OUTPUT_DIR ?? `test-results/${process.env.WS ?? "default"}`,
  reporter: [["list"], ["html", { open: "never", outputFolder: `playwright-report/${process.env.WS ?? "default"}` }]],
  use: {
    baseURL: fixtures.urls.web,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
})
