// The collectors of the image's third-party licenses, run against stubs (M12/P5/S2, fix round 1): tools/licenses/go.sh
// with a fake go and tools/licenses/npm.mjs with a fake pnpm, each cell in a directory of its own outside the
// repository. A cell wants the exit, words of the outcome, and on success INDEX.tsv and the files copied, whole: a name
// the pattern takes wrongly (cytoscape's license-update.mjs), a module whose only license-like name is code
// (license.go), a module the program holds that go.sh left out, pnpm asked for less than the workspace's packages'
// dependencies, or tailwindcss gone each turn a cell red. The fakes answer only the calls the scripts are to make.
// make lint-web runs it; it needs bash and Node, no Go and no build.
// Usage: node tools/licenses/check.mjs
import { spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");

/** write puts files ({ "relative/path": content }) under dir; a content of { link } is a symlink to it instead. */
function write(dir, files) {
  for (const [path, content] of Object.entries(files)) {
    const at = join(dir, path);
    mkdirSync(dirname(at), { recursive: true });
    if (typeof content === "object") {
      symlinkSync(content.link, at);
    } else {
      writeFileSync(at, content, { mode: path.startsWith("bin/") ? 0o755 : 0o644 });
    }
  }
}

/** filesUnder are the files under dir, relative to it, sorted. */
function filesUnder(dir) {
  return readdirSync(dir, { recursive: true })
    .filter((path) => statSync(join(dir, path)).isFile())
    .toSorted();
}

/** run executes the repository's script in cwd with the stub's bin (in dir) first in PATH and env besides. */
function run(dir, cwd, command, args, env) {
  const r = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    env: { ...process.env, PATH: `${join(dir, "bin")}:${process.env.PATH}`, ...env },
    timeout: 60_000,
  });
  return { code: r.status, stdout: r.stdout, stderr: r.stderr, error: r.error };
}

/**
 * The fake go: go list -deps (with CGO_ENABLED=0, as the build) prints the stub's list, go env its GOROOT and version,
 * and go version -m the program's first line, with the version or FAKE_GO_BUILT_WITH, and the stub's lines after it.
 */
const fakeGo = `#!/bin/sh
case "$*" in
"list -deps -f "*" ./cmd/nervewiki") [ "$CGO_ENABLED" = 0 ] || exit 2; cat "$FAKE_GO/list" ;;
"env GOROOT") echo "$FAKE_GO/goroot" ;;
"env GOVERSION") echo go1.99.9 ;;
"version -m "*) [ $# -eq 3 ] || exit 2; echo "$3: \${FAKE_GO_BUILT_WITH:-go1.99.9}"; cat "$FAKE_GO/info" ;;
*) echo "fake go: $*" >&2; exit 2 ;;
esac
`;

/** The modules of the stub, by id: their root's files. */
const MODULES = {
  "example.com/a@v1.0.0": {
    LICENSE: "a's license\n",
    "README.md": "a\n",
    "license.go": "package a\n",
    "copy_from.go": "package a\n",
    "LICENSE.d/MIT.txt": "a's other\n",
  },
  "example.com/b@v1.2.0": {
    "LICENSE-MIT": "b's MIT\n",
    "Notice.md": "b's notice\n",
    PATENTS: "b's patents\n",
    UNLICENSE: "b's unlicense\n",
    "copying.txt": "b's copying\n",
    licence: "b's licence\n",
    "license-update.mjs": "// b's script\n",
    "LICENSE.json": "{}\n",
  },
  "example.com/c@v1.0.0": { "license.go": "package c\n", COPYRIGHT: "c's copyright\n" },
};

/** goCell runs go.sh with the modules list names (each in the cache unless out of it) and the program holding held. */
function goCell({ name, list, held, outOfCache = [], env = {}, goroot = { LICENSE: "Go's\n", PATENTS: "Go's\n" } }) {
  const dir = mkdtempSync(join(tmpdir(), "licenses-go-"));
  const files = { "bin/go": fakeGo, "server/.keep": "" };
  for (const [id, content] of Object.entries(MODULES)) {
    for (const [path, text] of Object.entries(content)) files[`mod/${id}/${path}`] = text;
  }
  for (const [path, text] of Object.entries(goroot)) files[`goroot/${path}`] = text;
  files.list = list.map((id) => `${id}\t${outOfCache.includes(id) ? "" : join(dir, "mod", id)}\n`).join("");
  files.info = [
    "\tpath\texample.com/nervewiki/cmd/nervewiki",
    "\tmod\texample.com/nervewiki\t(devel)\t",
    ...held.map((id) => `\tdep\t${id.replace("@", "\t")}\th1:stub=`),
    "\tbuild\t-buildmode=exe",
  ]
    .map((line) => `${line}\n`)
    .join("");
  write(dir, files);
  const r = run(
    dir,
    join(dir, "server"),
    join(root, "tools/licenses/go.sh"),
    [join(dir, "nervewiki"), join(dir, "out")],
    {
      FAKE_GO: dir,
      ...env,
    }
  );
  return { name, dir, out: join(dir, "out"), ...r };
}

/** The fake pnpm answers the one listing npm.mjs is to ask for, the stub's, and nothing else. */
const fakePnpm = `#!/bin/sh
[ "$*" = "--filter @nervewiki/web... licenses list --prod --json" ] || { echo "fake pnpm: $*" >&2; exit 2; }
cat "$FAKE_PNPM_LISTING"
`;

/** pkg is a package.json. */
function pkg(name, version, license) {
  return JSON.stringify({ name, version, ...(license ? { license } : {}) });
}

/** npmCell runs npm.mjs in a stub repository whose pnpm lists listed (its paths relative) and tailwindcss installed or not. */
function npmCell({ name, listed, tailwind = true }) {
  const dir = mkdtempSync(join(tmpdir(), "licenses-npm-"));
  const files = {
    "bin/pnpm": fakePnpm,
    "pkgs/a/package.json": pkg("a", "1.0.0"),
    "pkgs/a/LICENSE": "a's license\n",
    "pkgs/a/license-update.mjs": "// a's script\n",
    "pkgs/a/README.md": "a\n",
    "pkgs/b/package.json": pkg("b", "2.0.0"),
    "pkgs/c/package.json": pkg("@s/c", "3.0.0"),
    "pkgs/c/license.md": "c's license\n",
    "pkgs/c/UNLICENSE": "c's unlicense\n",
    "pkgs/c/Licence.ts": "export {};\n",
    "pkgs/c/notice.json": "{}\n",
    "pkgs/c-peer/package.json": pkg("@s/c", "3.0.0"),
    "pkgs/c-peer/license.md": "c's license\n",
    "pkgs/tw/package.json": pkg("tailwindcss", "4.0.0", "MIT"),
    "pkgs/tw/LICENSE": "tailwindcss's license\n",
  };
  if (tailwind) files["web/apps/web/node_modules/tailwindcss"] = { link: "../../../../pkgs/tw" };
  const listing = {};
  for (const [license, packages] of Object.entries(listed)) {
    listing[license] = packages.map(({ name: n, versions, paths }) => ({
      name: n,
      versions,
      license,
      paths: paths.map((path) => join(dir, path)),
    }));
  }
  files["listing.json"] = JSON.stringify(listing);
  write(dir, files);
  const r = run(dir, dir, process.execPath, [join(root, "tools/licenses/npm.mjs"), join(dir, "out")], {
    FAKE_PNPM_LISTING: join(dir, "listing.json"),
  });
  return { name, dir, out: join(dir, "out"), ...r };
}

const both = ["example.com/a@v1.0.0", "example.com/b@v1.2.0"];
const listed = {
  MIT: [
    { name: "a", versions: ["1.0.0"], paths: ["pkgs/a"] },
    { name: "b", versions: ["2.0.0"], paths: ["pkgs/b"] },
  ],
  "Apache-2.0": [{ name: "@s/c", versions: ["3.0.0"], paths: ["pkgs/c", "pkgs/c-peer"] }],
};

/** Each cell: what ran, then the exit, words in its output, and on success INDEX.tsv and the files under out. */
const CELLS = [
  {
    ...goCell({ name: "go.sh: each module's license files and Go itself's", list: both, held: both }),
    want: 0,
    words: "3 行写进",
    index: [
      "example.com/a@v1.0.0\tLICENSE",
      "example.com/b@v1.2.0\tLICENSE-MIT,Notice.md,PATENTS,UNLICENSE,copying.txt,licence",
      "go@go1.99.9\tLICENSE,PATENTS",
    ],
    files: [
      "INDEX.tsv",
      "example.com/a@v1.0.0/LICENSE",
      "example.com/b@v1.2.0/LICENSE-MIT",
      "example.com/b@v1.2.0/Notice.md",
      "example.com/b@v1.2.0/PATENTS",
      "example.com/b@v1.2.0/UNLICENSE",
      "example.com/b@v1.2.0/copying.txt",
      "example.com/b@v1.2.0/licence",
      "go@go1.99.9/LICENSE",
      "go@go1.99.9/PATENTS",
    ],
  },
  {
    ...goCell({
      name: "go.sh: a module whose one license-like name is license.go",
      list: ["example.com/a@v1.0.0", "example.com/c@v1.0.0"],
      held: ["example.com/a@v1.0.0", "example.com/c@v1.0.0"],
    }),
    want: 1,
    words: "模块 example.com/c@v1.0.0 没有许可证文件",
  },
  {
    ...goCell({ name: "go.sh: a module out of the cache", list: both, held: both, outOfCache: [both[1]] }),
    want: 1,
    words: "模块 example.com/b@v1.2.0 不在模块缓存里",
  },
  {
    ...goCell({ name: "go.sh: the program holds a module it did not take", list: [both[0]], held: both }),
    want: 1,
    words: "只在程序里的 [example.com/b@v1.2.0]",
  },
  {
    ...goCell({ name: "go.sh: it took a module the program does not hold", list: both, held: [both[0]] }),
    want: 1,
    words: "只在 go list 里的 [example.com/b@v1.2.0]",
  },
  {
    ...goCell({
      name: "go.sh: the program built by another Go",
      list: both,
      held: both,
      env: { FAKE_GO_BUILT_WITH: "go1.99.8" },
    }),
    want: 1,
    words: "不是 go1.99.9 构建的",
  },
  {
    ...goCell({ name: "go.sh: Go itself without PATENTS", list: both, held: both, goroot: { LICENSE: "Go's\n" } }),
    want: 1,
    words: "Go 本身没有 PATENTS",
  },
  {
    ...npmCell({ name: "npm.mjs: the production dependencies and tailwindcss", listed }),
    want: 0,
    words: "4 packages",
    index: [
      "@s/c@3.0.0\tApache-2.0\tUNLICENSE,license.md",
      "a@1.0.0\tMIT\tLICENSE",
      "b@2.0.0\tMIT\t-",
      "tailwindcss@4.0.0\tMIT\tLICENSE",
    ],
    files: [
      "@s/c@3.0.0/UNLICENSE",
      "@s/c@3.0.0/license.md",
      "INDEX.tsv",
      "a@1.0.0/LICENSE",
      "tailwindcss@4.0.0/LICENSE",
    ],
  },
  {
    ...npmCell({ name: "npm.mjs: tailwindcss not installed", listed, tailwind: false }),
    want: 1,
    words: "tailwindcss, whose code ships",
  },
  {
    ...npmCell({ name: "npm.mjs: pnpm lists nothing", listed: {} }),
    want: 1,
    words: "pnpm listed no production dependency",
  },
];

const problems = [];
for (const cell of CELLS) {
  const said = `${cell.stdout ?? ""}${cell.stderr ?? ""}`;
  if (cell.error) {
    problems.push(`${cell.name}: did not run: ${cell.error.message}`);
  } else if (cell.code !== cell.want || !said.includes(cell.words)) {
    problems.push(`${cell.name}: exit ${cell.code}, want ${cell.want} and "${cell.words}"; it said: ${said.trim()}`);
  } else if (cell.want === 0) {
    const index = readFileSync(join(cell.out, "INDEX.tsv"), "utf8");
    if (index !== `${cell.index.join("\n")}\n`) {
      problems.push(`${cell.name}: INDEX.tsv is\n${index}want\n${cell.index.join("\n")}`);
    }
    const files = filesUnder(cell.out);
    if (JSON.stringify(files) !== JSON.stringify(cell.files)) {
      problems.push(`${cell.name}: copied ${JSON.stringify(files)}, want ${JSON.stringify(cell.files)}`);
    }
  }
  rmSync(cell.dir, { recursive: true, force: true });
}
if (problems.length > 0) {
  for (const problem of problems) console.error(problem);
  process.exit(1);
}
console.log(`licenses: ${CELLS.length} cells of go.sh and npm.mjs against stubs pass`);
