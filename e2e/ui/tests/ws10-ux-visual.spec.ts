import { test, expect, openAs, PAGES, heading } from "./ws10-helpers"
import { stubApi, FIXED_NOW } from "./ws10-stub"

// WS-10: visual baselines of every page as admin and viewer at 1280x800.
// Data comes from ws10-stub (canned, deterministic) and the clock is frozen,
// so "x minutes ago" and counts are stable; residual dynamic regions are masked.
// Update baselines: WS=ws10 npx playwright test tests/ws10-ux-visual --update-snapshots

test.use({ viewport: { width: 1280, height: 800 } })
test.describe.configure({ timeout: 60_000 })

for (const persona of ["admin", "viewer"] as const) {
  test.describe(`WS-10 visual (${persona})`, () => {
    for (const p of PAGES) {
      test(`${p.path} matches baseline`, async ({ page }) => {
        await page.clock.setFixedTime(FIXED_NOW)
        await openAs(page, persona, "/login")
        await stubApi(page)
        await page.goto(p.path)
        await expect(heading(page, p.heading)).toBeVisible()
        await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0)
        // Charts animate in; wait for fonts + a settled frame.
        await page.evaluate(() => document.fonts.ready)
        await page.waitForTimeout(800)
        const name = `${persona}${p.path === "/" ? "-dashboard" : p.path.replace(/\//g, "-")}.png`
        await expect(page).toHaveScreenshot(name, {
          animations: "disabled",
          caret: "hide",
          maxDiffPixelRatio: 0.01,
          mask: [
            // Locale-dependent dates and relative times.
            page.locator("td").filter({ hasText: /\d{1,2}\/\d{1,2}\/\d{2,4}|ago$|^just now$/ }),
            // Recharts output (sub-pixel antialiasing differs between runs).
            page.locator(".recharts-wrapper"),
          ],
        })
      })
    }
  })
}
