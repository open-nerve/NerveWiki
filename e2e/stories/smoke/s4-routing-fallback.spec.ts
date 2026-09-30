import { emailFor, password, registerOnboarded } from "../../fixtures/auth";
import { signInWith } from "../../fixtures/auth-pages";
import { expect, test } from "../../fixtures/test";

/** A path that is no page of the app, nor a file: nervewiki answers it with index.html. */
const deepLink = "/acme/notebooks/1";

test("S4: a user opens a deep link that is no page, signs in, and gets the app's 404, reload included", async ({
  page,
  pageWatch,
  request,
  api,
}, testInfo) => {
  const email = emailFor(testInfo);
  await registerOnboarded(api, email);

  const document = await page.goto(deepLink);

  expect(document?.status()).toBe(200);
  expect(document?.headers()["content-type"]).toBe("text/html; charset=utf-8");
  expect(await document?.body()).toEqual(await (await request.get("/")).body());
  // Every page needs a session, an unknown one too: the sign-in page comes back to it, and asks the API
  // for nothing but the instance information.
  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await expect(page).toHaveURL(`/sign-in?next=${encodeURIComponent(deepLink)}`);
  expect(pageWatch.apiRequests).toEqual(["GET /api/v0/instance"]);

  await signInWith(page, email, password);

  const notFound = page.getByRole("heading", { level: 1, name: "Page not found" });
  await expect(notFound).toBeVisible();
  await expect(page).toHaveURL(deepLink);
  await page.reload();
  await expect(page).toHaveURL(deepLink);
  await expect(notFound).toBeVisible();
  expect(pageWatch.apiFailures).toEqual([]);
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
