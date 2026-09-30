import { expect, stampedVersion, test } from "../../fixtures/test";

test("S3: a caller reads the instance information with the typed client", async ({ api }) => {
  const { data, error, response } = await api.GET("/api/v0/instance");

  expect(response.status).toBe(200);
  expect(error).toBeUndefined();
  expect(data).toEqual({
    product: "Nerve Wiki",
    version: stampedVersion(),
    commit: expect.stringMatching(/^[0-9a-f]{40}$/),
    api_version: "v0",
    // The test configuration opens sign-up; prod's closes it.
    signup_enabled: true,
  });
});
