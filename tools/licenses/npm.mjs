// The image's third-party licenses, the npm side (M12/P5/S2, P5 design 3.11). deploy/Dockerfile's web stage runs it in
// /src, the repository's root, after pnpm install and before the web app's source comes in, so that the layer changes
// with the lockfile alone:
//   node tools/licenses/npm.mjs <output directory>
// Every production dependency of @nervewiki/web and of the workspace's packages it takes in (@nervewiki/api-client's
// openapi-fetch is bundled too), as pnpm lists them, and the devDependencies whose code ships all the same (SHIPPED),
// gets a directory <name>@<version> holding the files of its root whose names look like a license's (the names
// tools/licenses/go.sh takes). A package with none is written down as "-", not a failure (P5 design P5-10: three
// today, whose package.json all say MIT). INDEX.tsv has a line <name>@<version> TAB <the license its package.json
// gives> TAB <the files, comma-separated, or "-">, sorted, so that one lockfile gives one directory. The name and
// version are read from each package's own package.json, not paired by place from pnpm's lists. tools/licenses/check.mjs
// runs it against a fake pnpm, deploy/image-smoke.sh checks the image's, and release.yml holds it to the web app's SBOM.
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  realpathSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";

/**
 * A license's file, in any case: LICENSE, LICENCE, UNLICENSE, COPYING, NOTICE or PATENTS, with a suffix such as .txt,
 * .md or -MIT or none, but not code or data (CODE: js, cjs, mjs, jsx, ts, cts, mts, tsx, go, json, map, sh, py), as
 * cytoscape's license-update.mjs is. go.sh's are the same two.
 */
const LICENSE_FILE = /^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$/i;
const CODE = /\.([cm]?[jt]sx?|go|json|map|sh|py)$/i;

/**
 * The devDependencies whose code ships in the build all the same: tailwindcss's preflight and default theme are
 * compiled into assets/index-*.css (web/apps/web/src/styles.css imports it), which keeps only its banner line. Vite's
 * modulepreload polyfill, a snippet of the build tool's own, is not counted. Each is read where pnpm installed it for
 * the web app.
 */
const SHIPPED = ["tailwindcss"];

if (process.argv.length !== 3) {
  console.error("usage: node tools/licenses/npm.mjs <output directory>");
  process.exit(1);
}
const out = process.argv[2];

const rows = new Map();

/** take writes down the package installed at path, under the license given, once for each name and version. */
function take(path, license) {
  const { name, version } = JSON.parse(readFileSync(join(path, "package.json"), "utf8"));
  const id = `${name}@${version}`;
  if (rows.has(id)) {
    return;
  }
  const files = readdirSync(path)
    .filter((file) => LICENSE_FILE.test(file) && !CODE.test(file) && statSync(join(path, file)).isFile())
    .toSorted();
  if (files.length > 0) {
    mkdirSync(join(out, id), { recursive: true });
    for (const file of files) {
      copyFileSync(join(path, file), join(out, id, file));
    }
  }
  rows.set(id, `${id}\t${license}\t${files.length > 0 ? files.join(",") : "-"}`);
}

/** pnpm's listing: { "<license>": [{ name, versions, paths, license, ... }] }. */
const listing = JSON.parse(
  execFileSync("pnpm", ["--filter", "@nervewiki/web...", "licenses", "list", "--prod", "--json"], {
    encoding: "utf8",
    maxBuffer: 64 << 20,
  })
);
for (const packages of Object.values(listing)) {
  for (const pkg of packages) {
    for (const path of pkg.paths) {
      take(path, pkg.license);
    }
  }
}
if (rows.size === 0) {
  console.error("licenses/npm.mjs: pnpm listed no production dependency of @nervewiki/web");
  process.exit(1);
}
for (const name of SHIPPED) {
  const link = join("web/apps/web/node_modules", name);
  if (!existsSync(link)) {
    console.error(`licenses/npm.mjs: ${name}, whose code ships in the web app's build, is not installed at ${link}`);
    process.exit(1);
  }
  const path = realpathSync(link);
  take(path, JSON.parse(readFileSync(join(path, "package.json"), "utf8")).license);
}
mkdirSync(out, { recursive: true });
const lines = [...rows.keys()].toSorted().map((id) => rows.get(id));
writeFileSync(join(out, "INDEX.tsv"), `${lines.join("\n")}\n`);
const bare = lines.filter((line) => line.endsWith("\t-")).length;
console.log(
  `licenses/npm.mjs: ${lines.length} packages in ${join(out, "INDEX.tsv")}, ${bare} without a license's text`
);
