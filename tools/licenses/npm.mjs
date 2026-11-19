// The image's third-party licenses, the npm side (M12/P5/S2, P5 design 3.11). deploy/Dockerfile's web stage runs it in
// /src after pnpm install and before the web app's source comes in, so that the layer changes with the lockfile alone:
//   node tools/licenses/npm.mjs <output directory>
// Every production dependency of @nervewiki/web, as pnpm lists them, gets a directory <name>@<version> holding the
// files of its root whose names look like a license's (the names tools/licenses/go.sh takes). A package with none is
// written down as "-", not a failure (P5 design P5-10: three today, whose package.json all say MIT). INDEX.tsv has a
// line <name>@<version> TAB <the license its package.json gives> TAB <the files, comma-separated, or "-">, sorted, so
// that one lockfile gives one directory. The name and version are read from each package's own package.json, not
// paired by place from pnpm's lists. deploy/image-smoke.sh checks the outcome.
import { execFileSync } from "node:child_process";
import { copyFileSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const LICENSE_FILE = /^(licen[cs]e|copying|notice|patents)([.-].*)?$/i;

if (process.argv.length !== 3) {
  console.error("usage: node tools/licenses/npm.mjs <output directory>");
  process.exit(1);
}
const out = process.argv[2];

/** pnpm's listing: { "<license>": [{ name, versions, paths, license, ... }] }. */
const listing = JSON.parse(
  execFileSync("pnpm", ["--filter", "@nervewiki/web", "licenses", "list", "--prod", "--json"], {
    encoding: "utf8",
    maxBuffer: 64 << 20,
  })
);

const rows = new Map();
for (const packages of Object.values(listing)) {
  for (const pkg of packages) {
    for (const path of pkg.paths) {
      const { name, version } = JSON.parse(readFileSync(join(path, "package.json"), "utf8"));
      const id = `${name}@${version}`;
      if (rows.has(id)) {
        continue;
      }
      const files = readdirSync(path)
        .filter((file) => LICENSE_FILE.test(file) && statSync(join(path, file)).isFile())
        .toSorted();
      if (files.length > 0) {
        mkdirSync(join(out, id), { recursive: true });
        for (const file of files) {
          copyFileSync(join(path, file), join(out, id, file));
        }
      }
      rows.set(id, `${id}\t${pkg.license}\t${files.length > 0 ? files.join(",") : "-"}`);
    }
  }
}
if (rows.size === 0) {
  console.error("licenses/npm.mjs: pnpm listed no production dependency of @nervewiki/web");
  process.exit(1);
}
mkdirSync(out, { recursive: true });
const lines = [...rows.keys()].toSorted().map((id) => rows.get(id));
writeFileSync(join(out, "INDEX.tsv"), `${lines.join("\n")}\n`);
const bare = lines.filter((line) => line.endsWith("\t-")).length;
console.log(
  `licenses/npm.mjs: ${lines.length} packages in ${join(out, "INDEX.tsv")}, ${bare} without a license's text`
);
