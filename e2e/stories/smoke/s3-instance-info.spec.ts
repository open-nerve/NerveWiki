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
    // Every profile lets accounts create workspaces.
    workspace_creation_enabled: true,
    // asset.max_bytes, 50 MiB in every profile.
    asset_max_bytes: 52_428_800,
    // transfer.export_ttl, 24 hours in every profile.
    export_ttl_seconds: 86_400,
    // transfer.import_max_bytes, 512 MiB in every profile.
    import_max_bytes: 536_870_912,
  });
});
