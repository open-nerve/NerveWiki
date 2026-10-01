import { createClient } from "@nervewiki/api-client";

import { countWorkspaces, expectNoWorkspaceAdded } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { slugFor } from "../../fixtures/workspaces";

// W2, the switch of workspace creation (M2 design 3).

test("W2 (API): with creation off, the instance says so and creating answers 403", async ({
  db,
  nervewikiWith,
}, testInfo) => {
  const closed = createClient({
    baseUrl: (await nervewikiWith(db.url, { env: { NWIKI_WORKSPACE__CREATION_ENABLED: "false" } })).baseURL,
  });
  const info = await closed.GET("/api/v0/instance");
  expect(info.data?.workspace_creation_enabled).toBe(false);

  const session = await register(closed, emailFor(testInfo));
  const pat = (await createToken(closed, session.access_token, { name: "W2" })).token;
  const before = await countWorkspaces(db);
  // The switch answers first, even to values that break the rules.
  const refusals = await Promise.all(
    [
      { name: "Acme", slug: slugFor(testInfo) },
      { name: "", slug: "Not A Slug" },
    ].map((body) => closed.POST("/api/v0/workspaces", { body, headers: bearer(pat) }))
  );
  for (const refused of refusals) {
    expect(refused.response.status).toBe(403);
    expect(refused.error?.code).toBe("workspace.creation_disabled");
  }
  await expectNoWorkspaceAdded(db, before);
});
