import { expect, test } from "../../fixtures/test";

test("S3: a caller reads the instance information with the typed client", async ({ api }) => {
  const version = process.env.NWIKI_E2E_VERSION;
  expect(
    version,
    "NWIKI_E2E_VERSION, the version make build stamped into bin/nervewiki (make e2e sets it)"
  ).toBeTruthy();

  const { data, error, response } = await api.GET("/api/v0/instance");

  expect(response.status).toBe(200);
  expect(error).toBeUndefined();
  expect(data).toEqual({
    product: "Nerve Wiki",
    version,
    commit: expect.stringMatching(/^[0-9a-f]{40}$/),
    api_version: "v0",
  });
});
