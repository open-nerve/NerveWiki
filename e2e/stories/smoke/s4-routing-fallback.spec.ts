import { expectQuietConsole, watchPage } from "../../fixtures/browser";
import { expect, test } from "../../fixtures/test";

/** A path that is no page of the app, nor a file: nervewiki answers it with index.html. */
const deepLink = "/acme/notebooks/1";

test("S4: a user opens a deep link that is no page and gets the app's 404, reload included", async ({
  page,
  request,
}) => {
  const watch = await watchPage(page);

  const document = await page.goto(deepLink);

  expect(document?.status()).toBe(200);
  expect(document?.headers()["content-type"]).toBe("text/html; charset=utf-8");
  expect(await document?.body()).toEqual(await (await request.get("/")).body());
  const notFound = page.getByRole("heading", { level: 1, name: "Page not found" });
  await expect(notFound).toBeVisible();

  await page.reload();
  await expect(page).toHaveURL(deepLink);
  await expect(notFound).toBeVisible();

  expect(watch.apiRequests).toEqual([]);
  expect(watch.cspViolations).toEqual([]);
  expect(watch.pageErrors).toEqual([]);
  await expectQuietConsole(page, watch);
});

// The typed client cannot express a path the API does not have, so the request goes out directly.
test("S4: a caller requests an API path that does not exist", async ({ request }) => {
  const res = await request.get("/api/v0/nope");

  expect(res.status()).toBe(404);
  expect(res.headers()["content-type"]).toBe("application/problem+json");
  expect(await res.json()).toEqual({
    status: 404,
    code: "not_found",
    title: "Not Found",
    detail: "no API endpoint for GET /api/v0/nope",
  });
});
