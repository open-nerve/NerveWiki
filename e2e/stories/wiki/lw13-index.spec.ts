/* oxlint-disable no-await-in-loop -- Pagination and fixture writes have a deliberate order. */
import { bearer, createToken, emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

for (const mode of ["standard", "llm_wiki"] as const) {
  test(`LW13: a reader pages and filters the ${mode} directory with complete types`, async ({
    api,
    nervewiki,
  }, info) => {
    const team = await newTeam(api, info);
    const notebook = await createNotebook(api, team.pat, team.workspace.slug, "Directory", "none", {
      mode,
      templateLocale: "en",
    });
    const notes: string[] = [];
    const topics: string[] = [];
    for (const [type, names] of [
      ["note", ["Alpha", "Beta", "Gamma"]],
      ["topic", ["Delta", "Epsilon"]],
    ] as const) {
      for (const name of names) {
        const page = await createPage(
          api,
          team.pat,
          notebook.id,
          name,
          null,
          `---\ntype: ${type}\nsummary: ${name}\ntags: [keep]\n---\n${name}`
        );
        (type === "note" ? notes : topics).push(page.id);
      }
    }
    if (notebook.source_root_id)
      await createPage(
        api,
        team.pat,
        notebook.id,
        "Source",
        notebook.source_root_id,
        "---\ntype: source-only\n---\nsource payload"
      );
    const reader = await joinAs(api, team.pat, team.workspace.slug, emailFor(info, "reader"), "guest");
    const me = await api.GET("/api/v0/me", { headers: bearer(reader) });
    if (!me.data) throw new Error("reader has no user identity");
    await addedNotebookMember(api, team.pat, notebook.id, me.data.id, "reader");
    const expectedTypes = [
      { type: "note", count: 3 },
      { type: "topic", count: 2 },
    ];
    for (const type of ["note", "topic"]) {
      let cursor: string | undefined;
      let ended = false;
      const ids: string[] = [];
      for (let page = 0; page < 5; page++) {
        const { data, response } = await api.GET("/api/v0/notebooks/{notebook_id}/index", {
          params: {
            path: { notebook_id: notebook.id },
            query: { type, tag: "keep", limit: 1, ...(cursor ? { cursor } : {}) },
          },
          headers: bearer(reader),
        });
        expect(response.status).toBe(200);
        if (!data) throw new Error("directory returned no body");
        expect(data.mode).toBe(mode);
        expect(data.types).toEqual(expectedTypes);
        expect(data.types_complete).toBe(true);
        expect(data.unfilterable_type_pages).toBe(0);
        expect(data.source_area).toEqual(mode === "llm_wiki" ? { root_id: notebook.source_root_id, sources: 1 } : null);
        expect(data.data).toHaveLength(1);
        expect(data.data[0]?.type).toBe(type);
        ids.push(data.data[0]!.page_id);
        if (data.next_cursor === null) {
          ended = true;
          break;
        }
        cursor = data.next_cursor;
      }
      expect(ended, "pagination terminates within the fixture's bound").toBe(true);
      expect(ids).toEqual(type === "note" ? notes : topics);
    }
    const pat = (await createToken(api, reader, { name: "LW13 reader" })).token;
    for (const scope of ["notebook", "account"] as const) {
      const client = await mcpClient(nervewiki.baseURL, pat, scope === "notebook" ? { notebook: notebook.id } : {});
      const response = resultOf<{ pages: { path: string }[] }>(
        await client.call("index", {
          ...(scope === "account" ? { notebook: notebook.id } : {}),
          type: "topic",
          tag: "keep",
        }),
        `${scope} reader index`
      );
      expect(response.pages.map((item) => item.path)).toEqual(["Delta.md", "Epsilon.md"]);
      expect(response).not.toHaveProperty("types");
    }
  });
}
