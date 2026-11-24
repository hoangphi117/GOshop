import { test, expect } from "@playwright/test";

test.describe("Authentication Flow", () => {
  test.beforeEach(async ({ page }) => {
    // Intercept backend auth API for reliable, fast hermetic E2E tests
    await page.route("**/api/v1/auth/login", async (route) => {
      const payload = route.request().postDataJSON();
      if (payload?.email === "wrong@example.com") {
        return route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({
            status: "error",
            message: "Invalid email or password",
          }),
        });
      }
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            accessToken: "mocked-jwt-access-token",
            access_token: "mocked-jwt-access-token",
            user: {
              id: 1,
              email: "admin@goshop.com",
              fullName: "System Admin",
              roles: ["admin"],
            },
          },
        }),
      });
    });

    await page.route("**/api/v1/users/profile", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            id: 1,
            email: "admin@goshop.com",
            full_name: "System Admin",
            avatar_url: "",
          },
        }),
      });
    });
  });

  test("should show validation error when submitting empty fields", async ({
    page,
  }) => {
    await page.goto("/login");
    await expect(page).toHaveURL(/.*login/);

    const submitBtn = page.getByRole("button", { name: /sign in/i });
    await submitBtn.click();

    await expect(
      page.getByText(/please enter your email address/i),
    ).toBeVisible();
  });

  test("should display error message on wrong credentials", async ({
    page,
  }) => {
    await page.goto("/login");

    await page.locator('input[type="email"]').fill("wrong@example.com");
    await page.locator('input[type="password"]').fill("WrongPassword123!");
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page.getByText(/Invalid email or password/i)).toBeVisible({
      timeout: 5000,
    });
  });

  test("should log in successfully with valid credentials and redirect", async ({
    page,
  }) => {
    await page.goto("/login");

    await page.locator('input[type="email"]').fill("admin@goshop.com");
    await page.locator('input[type="password"]').fill("Admin123!");
    await page.getByRole("button", { name: /sign in/i }).click();

    // Verify successful redirection
    await expect(page).not.toHaveURL(/.*login/, { timeout: 8000 });
  });
});
