import { expect, stampedVersion, test } from "../../fixtures/test";

/** The Content-Security-Policy of every page (server/internal/platform/webui/csp.go). */
const contentSecurityPolicy =
  "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
  "font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; " +
  "frame-ancestors 'none'";

function isStatic(url: string): boolean {
  return !new URL(url).pathname.startsWith("/api/");
}

test("S2: a user opens the home page and sees what the instance runs", async ({ page, pageWatch, api }) => {
  const { data: instance } = await api.GET("/api/v0/instance");

  const requested: string[] = [];
  const loaded: string[] = [];
  const failed: string[] = [];
  page.on("request", (req) => {
    requested.push(req.url());
  });
  // Static resources only: the API's requests and failures are watch's.
  page.on("response", (res) => {
    if (isStatic(res.url())) {
      (res.status() < 400 ? loaded : failed).push(`${res.status()} ${res.url()}`);
    }
  });
  page.on("requestfailed", (req) => {
    if (isStatic(req.url())) {
      failed.push(`${req.failure()?.errorText} ${req.url()}`);
    }
  });

  const document = await page.goto("/");

  expect(document?.status()).toBe(200);
  expect(document?.headers()["content-type"]).toBe("text/html; charset=utf-8");
  expect(document?.headers()["content-security-policy"]).toBe(contentSecurityPolicy);
  await expect(page.getByRole("heading", { level: 1, name: "Nerve Wiki" })).toBeVisible();
  await expect(page.getByText(`Version ${stampedVersion()} (${instance?.commit})`, { exact: true })).toBeVisible();

  // Everything comes from nervewiki, and all of it loads: the scripts, the styles, theme-init.js.
  const origin = new URL(page.url()).origin;
  expect(requested.filter((url) => new URL(url).origin !== origin)).toEqual([]);
  expect(loaded).toContainEqual(expect.stringMatching(/^200 .*\/assets\/[^/]+\.js$/));
  expect(loaded).toContainEqual(expect.stringMatching(/^200 .*\/theme-init\.js$/));
  expect(failed).toEqual([]);
  // The app asks nervewiki for one thing as it starts: the instance information. The page must also stay
  // quiet, which the page fixture checks as the test ends.
  expect(pageWatch.apiRequests).toEqual(["GET /api/v0/instance"]);
  expect(pageWatch.apiFailures).toEqual([]);
});
