import { test, expect, fx, loginAndWait } from "./helpers"

// Harness smoke: proves the console + IAMKit login + backend wiring work.
test.describe("harness smoke", () => {
  test("unauthenticated visit redirects to /login", async ({ page }) => {
    await page.goto("/providers")
    await expect(page).toHaveURL(/\/login/)
  })

  test("admin can log in and see the providers list", async ({ page }) => {
    await loginAndWait(page, fx.personas.admin)
    await page.goto("/providers")
    await expect(page.getByText("e2e-openai")).toBeVisible()
  })

  test("wrong password shows an error and stays on /login", async ({ page }) => {
    await page.goto("/login")
    await page.locator("#email").fill(fx.personas.admin.email!)
    await page.locator("#password").fill("definitely-wrong")
    await page.getByRole("button", { name: /sign in|log in|login/i }).click()
    await expect(page).toHaveURL(/\/login/)
    await expect(page.locator("p.text-destructive")).toBeVisible()
  })
})
