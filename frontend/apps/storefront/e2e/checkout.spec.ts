import { test, expect } from "@playwright/test";

test.describe("Checkout Flow with Product Variant", () => {
  test.beforeEach(async ({ page }) => {
    // 1. Mock Auth endpoints
    await page.route("**/api/v1/auth/refresh", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: { access_token: "mocked-jwt-token-123" },
        }),
      });
    });

    await page.route("**/api/v1/users/me", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            uuid: "11111111-1111-1111-1111-111111111111",
            email: "admin@goshop.com",
            full_name: "System Admin",
            phone_number: "0901234567",
          },
        }),
      });
    });

    // 2. Mock Cart & Notification endpoints
    await page.route("**/api/v1/cart**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            uuid: "cart-uuid-mock",
            total_items: 1,
            total_amount: 35340000,
            items: [
              {
                uuid: "cart-item-uuid-1",
                product_uuid: "11111111-2222-3333-4444-555555555555",
                variant_uuid: "66666666-7777-8888-9999-000000000000",
                product_name: "iPhone 16 Pro Max 256GB Titanium",
                product_slug: "iphone-16-pro-max",
                product_image: "",
                price: 35340000,
                quantity: 1,
                subtotal: 35340000,
                stock_available: 10,
                is_available: true,
              },
            ],
          },
        }),
      });
    });

    await page.route("**/api/v1/notifications**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "success", data: [] }),
      });
    });

    // 3. Mock Address and Shipping endpoints
    await page.route("**/api/v1/addresses**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: [
            {
              uuid: "addr-uuid-1",
              label: "Home",
              name: "System Admin",
              phone: "0901234567",
              address: "123 Nguyen Hue Street",
              full_address:
                "123 Nguyen Hue Street, Ben Nghe Ward, District 1, Ho Chi Minh City",
              province_id: 201,
              province_name: "Ho Chi Minh City",
              district_id: 1442,
              district_name: "District 1",
              ward_code: "20101",
              ward_name: "Ben Nghe Ward",
              is_default: true,
            },
          ],
        }),
      });
    });

    await page.route("**/api/v1/shipping/provinces**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: [{ ProvinceID: 201, ProvinceName: "Ho Chi Minh City" }],
        }),
      });
    });

    await page.route("**/api/v1/shipping/districts**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: [
            {
              district_id: 1442,
              district_name: "District 1",
              province_id: 201,
            },
          ],
        }),
      });
    });

    await page.route("**/api/v1/shipping/wards**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: [
            {
              ward_code: "20101",
              ward_name: "Ben Nghe Ward",
              district_id: 1442,
            },
          ],
        }),
      });
    });

    await page.route("**/api/v1/shipping/calculate-fee**", async (route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: { shipping_fee: 30000, total: 30000 },
        }),
      });
    });

    // 4. Mock Order Creation endpoint
    await page.route("**/api/v1/orders**", async (route) => {
      const payload = route.request().postDataJSON();
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            uuid: "order-uuid-mock-999",
            order_number: "ORD-20261124-MOCK",
            subtotal_amount: 35340000,
            shipping_fee: 30000,
            discount_amount: 0,
            total_amount: 35370000,
            items: payload?.items || [],
          },
        }),
      });
    });
  });

  test("should load checkout page with variant item and place order showing total amount", async ({
    page,
  }) => {
    // Pre-populate localStorage so stores hydrate with user session & cart item
    await page.addInitScript(() => {
      const user = {
        id: 1,
        email: "admin@goshop.com",
        fullName: "System Admin",
        role: ["admin"],
        roles: ["admin"],
      };
      const cartItem = {
        id: "11111111-2222-3333-4444-555555555555-66666666-7777-8888-9999-000000000000",
        productUuid: "11111111-2222-3333-4444-555555555555",
        variantUuid: "66666666-7777-8888-9999-000000000000",
        name: "iPhone 16 Pro Max 256GB Titanium",
        price: 35340000,
        quantity: 1,
        image: "",
        sku: "IPH16PM-256-PRO",
        stockQuantity: 10,
        selected: true,
      };
      try {
        localStorage.setItem(
          "goshop_storefront_auth",
          JSON.stringify({ state: { user }, version: 0 }),
        );
        localStorage.setItem(
          "goshop_cart_1",
          JSON.stringify({ items: [cartItem], appliedCoupon: null }),
        );
        localStorage.setItem(
          "goshop_cart_null",
          JSON.stringify({ items: [cartItem], appliedCoupon: null }),
        );
      } catch {}
    });

    // Navigate directly to checkout page
    await page.goto("/checkout");
    await page.waitForLoadState("domcontentloaded");

    // Ensure access token is set in memory on the checkout page
    await page.evaluate(() => {
      const auth = (
        window as unknown as {
          __authStore?: {
            getState: () => {
              setTokens: (t: string) => void;
              setUser: (u: unknown) => void;
            };
          };
        }
      ).__authStore;
      if (auth) {
        auth.getState().setTokens("mocked-jwt-token-123");
        auth.getState().setUser({
          id: 1,
          email: "admin@goshop.com",
          fullName: "System Admin",
          role: ["admin"],
          roles: ["admin"],
        });
      }

      const cart = (
        window as unknown as {
          __cartStore?: {
            getState: () => {
              items: unknown[];
              addItem: (item: unknown) => void;
              closeDrawer: () => void;
            };
          };
        }
      ).__cartStore;
      if (cart) {
        if (!cart.getState().items || cart.getState().items.length === 0) {
          cart.getState().addItem({
            productUuid: "11111111-2222-3333-4444-555555555555",
            variantUuid: "66666666-7777-8888-9999-000000000000",
            name: "iPhone 16 Pro Max 256GB Titanium",
            price: 35340000,
            quantity: 1,
            image: "",
            sku: "IPH16PM-256-PRO",
            stockQuantity: 10,
          });
        }
        cart.getState().closeDrawer();
      }
    });

    // Close cart drawer if visible so it doesn't intercept clicks
    const closeCartBtn = page.getByRole("button", {
      name: /close cart drawer/i,
    });
    if (await closeCartBtn.isVisible()) {
      await closeCartBtn.click();
    }

    await expect(page).toHaveURL(/.*checkout/);

    // Verify items and delivery step are loaded
    await expect(
      page.getByText(/Shipping Address|Delivery/i).first(),
    ).toBeVisible({ timeout: 10000 });

    // Verify the product item is present in order summary
    await expect(
      page.getByText(/iPhone 16 Pro Max 256GB Titanium/i).first(),
    ).toBeVisible();

    // Ensure COD is selected
    const codRadio = page.locator('input[value="COD"]');
    if ((await codRadio.count()) > 0 && !(await codRadio.isChecked())) {
      await codRadio.check();
    }

    // Dismiss address modal if accidentally opened
    const closeDialogBtn = page.getByRole("button", {
      name: /close address dialog/i,
    });
    if (await closeDialogBtn.isVisible()) {
      await closeDialogBtn.click();
    }

    // Click Place Order
    const placeOrderBtn = page.getByRole("button", { name: /place order/i });
    await expect(placeOrderBtn).toBeVisible();
    await placeOrderBtn.click();

    // Verify order confirmation screen
    await expect(
      page.getByRole("heading", { name: /thank you for your order/i }),
    ).toBeVisible({ timeout: 10000 });

    // Verify Total Amount breakdown
    await expect(page.getByText(/Total Amount:/i)).toBeVisible();
    await expect(page.getByText(/Subtotal:/i)).toBeVisible();
    await expect(page.getByText(/Shipping Fee:/i)).toBeVisible();
  });
});

