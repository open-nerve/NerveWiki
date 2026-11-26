// The e2e's projects, checked (M12/P2/S3; M12/P2 design 5.1, decision 421). A project's testMatch, testIgnore or
// grep written wrong makes no cell red: the cells it should run are just not run. Playwright lists what each project
// runs (--list, which starts no browser and no server), and this check holds the listing to what the configuration
// means: webkit-narrow lists the cells tagged @narrow of the stories the three browsers do not run already (those
// webkit lists), every one of them and nothing else, among them the cells REQUIRED names. The listing names projects,
// files and cells, not browsers: that webkit-narrow runs on WebKit is its line in e2e/playwright.config.ts, which the
// check does not see. The check first runs on samples of each breach and fails unless it sees them, and fails unless
// the listing holds the cells REQUIRED names, so it cannot pass by listing nothing.
// Usage: node tools/e2e-projects/check.mjs
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const NARROW = "narrow";

/**
 * The cells each project is to list at least: [project, file under e2e/stories/, the start of its title]. A cell whose
 * tag is lost leaves both lists of the tagged, which the rules below compare, so only this names it. SE1's is the only
 * cell that fails without NativeSelect's contain-paint, and only on WebKit (M5-polish 33.1).
 */
const REQUIRED = [
  // The reading view's focus and scroll are each engine's (M12/P2/S4, 4.4.6).
  ["firefox", "page/pg15-reading-updates.spec.ts", "PG15 (page): the reading view updated in place"],
  ["webkit", "page/pg15-reading-updates.spec.ts", "PG15 (page): the reading view updated in place"],
  // The tree's row menu and the row shown brought into the column are each engine's (M13/P2/S2, decision 789).
  ["firefox", "page/pg16-tree-menu.spec.ts", "PG16 (page): a row's menu in the tree"],
  ["webkit", "page/pg16-tree-menu.spec.ts", "PG16 (page): a row's menu in the tree"],
  // The page's head, its right column and their focus are each engine's (M13/P4/S1, decision 879).
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head says"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head says"],
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head keeps its lines"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head keeps its lines"],
  // Long names broken, a lock's holder's and a version's author's: Firefox breaks no line at a hyphen before a digit
  // (UAX #14's HY × NU), and the cell goes red there alone (M13/P4/S1, review B's P4S1B-F1, review C's P4S1C-F2-M1).
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): long names break"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): long names break"],
  // A code block's head, its Copy and the clipboard, and a done task item are each engine's (M13/P4/S2, decision 879).
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): a code block's head"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): a code block's head"],
  // The edit's outline's first jump, its line measured as it is drawn, is each engine's; the trail's separators each on
  // the line of their step's name: Firefox broke the line after one before its step, and the cell went red there alone
  // (M13 closeout, review B's M13CB-I1 and M13CB-M2).
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): edited from the page's top"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): edited from the page's top"],
  ["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): the trail keeps each separator"],
  ["webkit", "page/pg17-page-head.spec.ts", "PG17 (page): the trail keeps each separator"],
  // The drawer of a phone's width, its focus and Escape, are each engine's (M13/P2/S3, decision 789).
  ["firefox", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the left column is a drawer"],
  ["webkit", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the left column is a drawer"],
  // The cards' grid of the workspace's home, at 375 and 320, and a notebook's title broken are each engine's: an auto
  // track widened by a name cut short in the three, Firefox's breaks not the others' (M13/P5/S1, 5.1).
  ["firefox", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the workspace's home"],
  ["webkit", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the workspace's home"],
  // The entry's card and a page not found, their widths and the way home's focus, are each engine's, and so are the
  // invitations' cards breaking a name or an address that has no break (M13/P5/S1, 5.1, P5S1B-I1).
  ["firefox", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the sign-in card"],
  ["webkit", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the sign-in card"],
  // The settings' mode, their way in and out and its focus, are each engine's (M13/P3/S1, 5.3).
  ["firefox", "shell/ui2-settings-mode.spec.ts", "UI2 (page): the settings are entered"],
  ["webkit", "shell/ui2-settings-mode.spec.ts", "UI2 (page): the settings are entered"],
  // The access saved as it is chosen, its check's fade and the refusal's focus, are each engine's (M13/P3/S2, 5.5).
  ["firefox", "shell/ui3-instant-save.spec.ts", "UI3 (page): the workspace access goes out"],
  ["webkit", "shell/ui3-instant-save.spec.ts", "UI3 (page): the workspace access goes out"],
  ["webkit-narrow", "mcp/mc3-write-edit.spec.ts", "MC3 (page): at 375 wide"],
  ["webkit-narrow", "page/pg2-rename-page.spec.ts", "PG2 (page): at 375 wide"],
  ["webkit-narrow", "search/se1-search.spec.ts", "SE1 (page): at 375 wide"],
];

/** The cells of a listing (Playwright's JSON report), as { project, file, title, tags }. */
function cellsOf(report) {
  const cells = [];
  const walk = (suite) => {
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests) {
        cells.push({ project: test.projectName, file: spec.file, title: spec.title, tags: spec.tags ?? [] });
      }
    }
    for (const child of suite.suites ?? []) walk(child);
  };
  for (const suite of report.suites ?? []) walk(suite);
  return cells;
}

/** A cell's file and title, as Playwright's list writes them. */
const key = (cell) => `${cell.file} › ${cell.title}`;

/** The breaches in cells, against required: a list of "<project> <file>: <why>". */
function check(cells, required) {
  const problems = [];
  const of = (project) => cells.filter((cell) => cell.project === project);
  const threeBrowsers = new Set(of("webkit").map((cell) => cell.file));
  const narrow = of("webkit-narrow");
  for (const cell of narrow) {
    if (!cell.tags.includes(NARROW)) problems.push(`webkit-narrow ${key(cell)}: is not tagged @${NARROW}`);
    if (threeBrowsers.has(cell.file)) problems.push(`webkit-narrow ${key(cell)}: webkit runs its story already`);
  }
  const listed = new Set(narrow.map(key));
  for (const cell of of("chromium")) {
    if (cell.tags.includes(NARROW) && !threeBrowsers.has(cell.file) && !listed.has(key(cell))) {
      problems.push(`webkit-narrow ${key(cell)}: is tagged @${NARROW} but not listed`);
    }
  }
  for (const [project, file, start] of required) {
    if (!of(project).some((cell) => cell.file === file && cell.title.startsWith(start))) {
      problems.push(`${project} ${file}: lists no cell "${start}…"`);
    }
  }
  return problems;
}

// Each breach the check is to see, with the problems it is to report.
const mc3 = { file: "mcp/mc3-write-edit.spec.ts", title: "MC3 (page): at 375 wide, x", tags: [NARROW] };
const lw3 = { file: "wiki/lw3-source-duplicates.spec.ts", title: "LW3 (page): y", tags: [] };
const fine = [
  { project: "chromium", ...mc3 },
  { project: "webkit-narrow", ...mc3 },
  { project: "chromium", ...lw3 },
  { project: "webkit", ...lw3 },
];
const sampleRequired = [["webkit-narrow", mc3.file, "MC3 (page): at 375 wide"]];
const SAMPLES = [
  { name: "a listing as configured", cells: fine, want: [] },
  {
    name: "a cell not tagged (a grep too wide)",
    cells: [...fine, { project: "webkit-narrow", ...lw3, file: "page/pg9-x.spec.ts" }],
    want: ["webkit-narrow page/pg9-x.spec.ts › LW3 (page): y: is not tagged @narrow"],
  },
  {
    name: "a cell of a story webkit runs (no testIgnore)",
    cells: [...fine, { project: "webkit-narrow", ...lw3, tags: [NARROW] }],
    want: ["webkit-narrow wiki/lw3-source-duplicates.spec.ts › LW3 (page): y: webkit runs its story already"],
  },
  {
    name: "a tagged cell not run on WebKit, the required one (a grep too narrow)",
    cells: fine.filter((cell) => cell.project !== "webkit-narrow"),
    want: [
      "webkit-narrow mcp/mc3-write-edit.spec.ts › MC3 (page): at 375 wide, x: is tagged @narrow but not listed",
      'webkit-narrow mcp/mc3-write-edit.spec.ts: lists no cell "MC3 (page): at 375 wide…"',
    ],
  },
  {
    name: "the required cell's tag lost",
    cells: [
      { project: "chromium", ...mc3, tags: [] },
      { project: "chromium", ...lw3 },
      { project: "webkit", ...lw3 },
    ],
    want: ['webkit-narrow mcp/mc3-write-edit.spec.ts: lists no cell "MC3 (page): at 375 wide…"'],
  },
];

const problems = [];
for (const { name, cells, want } of SAMPLES) {
  const got = check(cells, sampleRequired);
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    problems.push(`the check does not see ${name}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
  }
}

const report = JSON.parse(
  execFileSync("pnpm", ["--filter", "@nervewiki/e2e", "exec", "playwright", "test", "--list", "--reporter=json"], {
    cwd: root,
    encoding: "utf8",
    maxBuffer: 64 << 20,
    stdio: ["ignore", "pipe", "inherit"],
  })
);
for (const error of report.errors ?? []) problems.push(`the listing: ${error.message ?? JSON.stringify(error)}`);
const cells = cellsOf(report);
problems.push(...check(cells, REQUIRED));
if (problems.length > 0) {
  for (const problem of problems) console.error(problem);
  process.exit(1);
}
const count = (project) => cells.filter((cell) => cell.project === project).length;
console.log(
  `e2e projects: ${["chromium", "firefox", "webkit", "webkit-narrow"].map((p) => `${p} ${count(p)}`).join(", ")}; webkit-narrow runs the cells tagged @${NARROW} webkit does not`
);
