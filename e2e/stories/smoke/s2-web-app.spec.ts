import { expectQuietConsole, watchPage } from "../../fixtures/browser";
import { expect, test } from "../../fixtures/test";

/** The Content-Security-Policy of every page (server/internal/platform/webui/csp.go). */
const contentSecurityPolicy =
  "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
  "font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; " +
  "frame-ancestors 'none'";

test("S2: a user opens the home page and sees what the instance runs", async ({ page, api }) => {
  const version = process.env.NWIKI_E2E_VERSION;
  expect(
    version,
    "NWIKI_E2E_VERSION, the version make build stamped into bin/nervewiki (make e2e sets it)"
  ).toBeTruthy();
  const { data: instance } = await api.GET("/api/v0/instance");

  const watch = await watchPage(page);
  const requested: string[] = [];
  const loaded: string[] = [];
  const failed: string[] = [];
  page.on("request", (req) => {
    requested.push(req.url());
  });
  page.on("response", (res) => {
    (res.status() < 400 ? loaded : failed).push(`${res.status()} ${res.url()}`);
  });
  page.on("requestfailed", (req) => {
    failed.push(`${req.failure()?.errorText} ${req.url()}`);
  });

  const document = await page.goto("/");

  expect(document?.status()).toBe(200);
  expect(document?.headers()["content-type"]).toBe("text/html; charset=utf-8");
  expect(document?.headers()["content-security-policy"]).toBe(contentSecurityPolicy);
  await expect(page.getByRole("heading", { level: 1, name: "Nerve Wiki" })).toBeVisible();
  await expect(page.getByText(`Version ${version} (${instance?.commit})`, { exact: true })).toBeVisible();

  // Everything comes from nervewiki, and all of it loads: the scripts, the styles, theme-init.js, the icon.
  const origin = new URL(page.url()).origin;
  expect(requested.filter((url) => new URL(url).origin !== origin)).toEqual([]);
  expect(loaded).toContainEqual(expect.stringMatching(/^200 .*\/assets\/[^/]+\.js$/));
  expect(loaded).toContainEqual(expect.stringMatching(/^200 .*\/theme-init\.js$/));
  expect(failed).toEqual([]);
  // The app asks nervewiki for one thing as it starts: the instance information.
  expect(watch.apiRequests).toEqual(["GET /api/v0/instance"]);
  expect(watch.apiFailures).toEqual([]);
  expect(watch.cspViolations).toEqual([]);
  expect(watch.pageErrors).toEqual([]);
  await expectQuietConsole(page, watch);
});
