// The e2e's gate on leaving a document (00-M12 4.4, M12/P1/S1). A document left while its refresh is out
// loses the refreshed token, and the next document ends the session as reuse. Only e2e/fixtures/navigation.ts
// calls page.goto and page.reload; everything else under e2e/ goes through its goto and reload, which wait
// first. A story that leaves mid-request on purpose passes the escape: LEAVES lists each file that does, how
// many times, and why. The escape counts written with any quote, a backtick too. The helper's module is read
// but not checked: its calls are the ones allowed, and its message names the escape; the exception is its whole
// path, not every module of that name. Elsewhere comments and strings count too, so they name it in words, not
// in its form. It reads every file with an extension Playwright runs by default (.ts, .tsx, .mts, .cts and the
// .js ones). Closing a tab leaves a document as well; the helper does not cover it (the stories close tabs
// once a page showed what only a signed-in session reads). The check first runs on samples of each breach and
// on the extensions, and fails unless it sees them, and fails unless the tree it read holds the helper's
// module, so it cannot pass by reading nothing, or by missing e2e/fixtures/ or the stories the table names.
// Usage: node tools/e2e-navigation/check.mjs
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const HELPER = "e2e/fixtures/navigation.ts";
const SKIP = new Set(["node_modules", "test-results", "playwright-report"]);
// Playwright's default testMatch is **/*.@(spec|test).?(c|m)[jt]s?(x); e2e/playwright.config.ts narrows it
// only for Firefox and WebKit.
const SOURCE = /\.[cm]?[jt]sx?$/;
const DIRECT = /\.(goto|reload)\s*\(/g;
const LEAVE = /inFlight:\s*["'`]leave["'`]/g;

/** Each file under e2e/ that leaves a document mid-request on purpose, and how many times. */
const LEAVES = {
  // A15's two tests of what the helper prevents: one leaves while the refresh's answer is out, one before
  // the document's first refresh has gone out (Chromium and Firefox only); both land on /sign-in (M12/P1/S1).
  "e2e/stories/identity/a15-leave-mid-refresh.spec.ts": 2,
};

/** The line of text that index falls on, counting from 1. */
function lineOf(text, index) {
  return text.slice(0, index).split("\n").length;
}

/** The breaches in files ({ path, text }), against leaves: a list of "<path>:<line>: <why>". */
function check(files, leaves) {
  const problems = [];
  const counted = {};
  for (const { path, text } of files) {
    if (path === HELPER) continue;
    for (const m of text.matchAll(DIRECT)) {
      problems.push(`${path}:${lineOf(text, m.index)}: calls ${m[1]} directly; use goto or reload from ${HELPER}`);
    }
    const leaving = [...text.matchAll(LEAVE)].length;
    if (leaving > 0) counted[path] = leaving;
  }
  for (const path of new Set([...Object.keys(counted), ...Object.keys(leaves)])) {
    if ((counted[path] ?? 0) !== (leaves[path] ?? 0)) {
      problems.push(`${path}: leaves mid-request ${counted[path] ?? 0} times, LEAVES says ${leaves[path] ?? 0}`);
    }
  }
  return { problems, counted };
}

// Each breach the check is to see, and the two things it lets be, with the problems it is to report.
const story = "e2e/stories/sample/x1-sample.spec.ts";
const fixture = "e2e/fixtures/sample.ts";
const SAMPLES = [
  {
    name: "a story's direct goto",
    files: [{ path: story, text: 'await page.goto("/");' }],
    leaves: {},
    want: [`${story}:1: calls goto directly; use goto or reload from ${HELPER}`],
  },
  {
    name: "a direct reload across lines in a fixture besides the helper",
    files: [{ path: fixture, text: "const tab = page;\nawait tab\n  .reload();" }],
    leaves: {},
    want: [`${fixture}:3: calls reload directly; use goto or reload from ${HELPER}`],
  },
  {
    name: "the helper's own call",
    files: [{ path: HELPER, text: "return page.goto(url, rest);" }],
    leaves: {},
    want: [],
  },
  {
    name: "an escape in a file LEAVES does not list",
    files: [{ path: story, text: 'await goto(page, "/", { inFlight: "leave" });' }],
    leaves: {},
    want: [`${story}: leaves mid-request 1 times, LEAVES says 0`],
  },
  {
    name: "a count LEAVES gets wrong",
    files: [{ path: story, text: 'await goto(page, "/", { inFlight: "leave" });' }],
    leaves: { [story]: 2 },
    want: [`${story}: leaves mid-request 1 times, LEAVES says 2`],
  },
  {
    name: "a LEAVES row for a file that no longer leaves",
    files: [{ path: story, text: 'await goto(page, "/");' }],
    leaves: { [story]: 1 },
    want: [`${story}: leaves mid-request 0 times, LEAVES says 1`],
  },
  {
    name: "the helper's message naming the escape",
    files: [{ path: HELPER, text: "throw new Error('… passes { inFlight: \"leave\" }');" }],
    leaves: {},
    want: [],
  },
  {
    name: "an escape written with backticks",
    files: [{ path: story, text: "await goto(page, `/`, { inFlight: `leave` });" }],
    leaves: {},
    want: [`${story}: leaves mid-request 1 times, LEAVES says 0`],
  },
  {
    name: "a direct goto in another module named like the helper",
    files: [{ path: "e2e/perf/navigation.ts", text: 'await page.goto("/");' }],
    leaves: {},
    want: [`e2e/perf/navigation.ts:1: calls goto directly; use goto or reload from ${HELPER}`],
  },
];

const problems = [];
for (const { name, files, leaves, want } of SAMPLES) {
  const got = check(files, leaves).problems;
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    problems.push(`the gate does not see ${name}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
  }
}
// Each ending Playwright's default testMatch takes.
for (const ext of ["ts", "tsx", "mts", "cts", "js", "jsx", "mjs", "cjs"]) {
  if (!SOURCE.test(`x1-sample.spec.${ext}`)) problems.push(`the gate does not read a story ending .spec.${ext}`);
}

/** Every source file under dir, as { path from the repository's root, text }. */
function sources(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = join(dir, entry.name);
    if (SKIP.has(entry.name)) return [];
    if (entry.isDirectory()) return sources(full);
    if (!SOURCE.test(entry.name)) return [];
    return [{ path: relative(root, full).split(sep).join("/"), text: readFileSync(full, "utf8") }];
  });
}

const files = sources(join(root, "e2e"));
// The samples go to check() directly: what proves that the tree was read where the helper's callers are is
// that its module was read with them, under e2e/ (the table's rows say the same for the stories).
if (!files.some((file) => file.path === HELPER)) {
  problems.push(`the gate did not read ${HELPER}: it reads every source file under e2e/`);
}
const real = check(files, LEAVES);
problems.push(...real.problems);
if (problems.length > 0) {
  for (const problem of problems) console.error(problem);
  process.exit(1);
}
const counts = Object.entries(real.counted).map(([path, k]) => `${path} ×${k}`);
const total = Object.values(real.counted).reduce((sum, k) => sum + k, 0);
console.log(
  `e2e navigation: ${files.length} files, no direct goto or reload; leaving mid-request: ${counts.join(", ") || "none"} (${total})`
);
