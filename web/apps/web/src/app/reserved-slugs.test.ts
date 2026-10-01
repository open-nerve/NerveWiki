import type { RouteObject } from "react-router";
import { expect, test } from "vitest";

import reserved from "../../../../../server/internal/modules/workspace/domain/reserved_slugs.txt?raw";
import { routes } from "./routes";

/** The names of a section of the reserved slugs, the server's one list (M2/P1 design 3.6). */
function section(name: string): string[] {
  const names: string[] = [];
  let current = "";
  for (const raw of reserved.split("\n")) {
    const line = raw.trim();
    if (line === "" || line.startsWith("#")) continue;
    if (line.startsWith("[")) current = line;
    else if (current === `[${name}]`) names.push(line);
  }
  return names;
}

/** The first segments of the paths routes serve at the top: a pathless route's children are at the top too. */
function topLevelSegments(list: RouteObject[]): string[] {
  return list.flatMap((route) =>
    route.path === undefined ? topLevelSegments(route.children ?? []) : [route.path.split("/")[0] ?? ""]
  );
}

test("the reserved slugs of the app are the top-level segments of its routes", () => {
  // A parameter (a workspace's slug) or the 404's * is not a name to reserve.
  const statics = topLevelSegments(routes).filter((s) => s !== "" && s !== "*" && !s.startsWith(":"));

  expect([...new Set(statics)].toSorted()).toEqual(section("app").toSorted());
});
