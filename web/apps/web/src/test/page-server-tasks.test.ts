import { expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { pageServer, pageNode } from "./page-server";

// The fake's task items, a line "- [ ]" or "- [x]" with a colon after "]", read as the server reads them (its
// goldmark parse and toggleTask's checks, probed in M12/P2/S2's review B): a colon and then a destination alone
// makes a link reference definition of "[x]", so "- [x]: /u" is no task, and "- [ ]: /u" ticked would leave none
// (422 not_allowed on offset); a colon with no destination, or more after it, is an ordinary task item. Not modelled:
// a definition with a title ("- [x]: /u \"t\"", no task to the server) and "- [ ]x" with no space, a task to it.

const notes = pageNode(40, "Notes");

/** the fake's tasks of content: each checkbox's offset and whether it is ticked, from the view withTasks gives. */
function tasksIn(server: ReturnType<typeof pageServer>): { offset: number; checked: boolean }[] {
  const html = server.views.get(notes.id)?.html ?? "";
  return [...html.matchAll(/<input ((?:checked="" )?)disabled="" type="checkbox" data-task="(\d+)">/g)].map(
    (match) => ({ offset: Number(match[2]), checked: match[1] !== "" })
  );
}

test.each([
  // [content, its tasks to the server, what a toggle at offset 3 does there]
  ["- [ ]: /u\n", [{ offset: 3, checked: false }], "not_allowed"],
  ["- [ ]: <>\n", [{ offset: 3, checked: false }], "not_allowed"],
  ["- [x]: /u\n", [], "out_of_range"],
  ["- [X]: /u\n", [], "out_of_range"],
  ["- [x]:/u\n", [], "out_of_range"],
  ["- [x]: <>\n", [], "out_of_range"],
  ["- [ ]:\n", [{ offset: 3, checked: false }], "written"],
  ["- [x]:\n", [{ offset: 3, checked: true }], "written"],
  ["- [ ]: \n", [{ offset: 3, checked: false }], "written"],
  ["- [x]: \n", [{ offset: 3, checked: true }], "written"],
  ["- [ ]: a b c\n", [{ offset: 3, checked: false }], "written"],
  ["- [x]: a b c\n", [{ offset: 3, checked: true }], "written"],
  ["- [ ]: /u x\n", [{ offset: 3, checked: false }], "written"],
  ["- [x]: /u x\n", [{ offset: 3, checked: true }], "written"],
  ["- [ ] b\n", [{ offset: 3, checked: false }], "written"],
] as const)(
  "a task line %j is, as the server reads it, %j, and a toggle at 3 is %s",
  async (content, tasks, toggled) => {
    const server = pageServer({ role: "editor", nodes: [notes] });
    const page = new PageService(server.app.session.public);
    server.withTasks(notes.id, content, 1);
    expect(tasksIn(server)).toEqual(tasks);

    const checked = !(tasks[0]?.checked ?? true);
    const answer = page.toggleTask(notes.id, { base_revision: 1, offset: 3, checked });
    if (toggled === "written") {
      await expect(answer).resolves.toMatchObject({ revision: 2 });
      const bytes = new TextEncoder().encode(content);
      bytes[3] = (checked ? "x" : " ").charCodeAt(0);
      expect(server.contents.get(notes.id)?.content).toBe(new TextDecoder().decode(bytes));
    } else {
      await expect(answer).rejects.toMatchObject({
        code: "validation_failed",
        status: 422,
        problem: { errors: [{ field: "offset", code: toggled }] },
      });
      expect(server.contents.get(notes.id)).toEqual({ content, revision: 1 });
    }
  }
);
