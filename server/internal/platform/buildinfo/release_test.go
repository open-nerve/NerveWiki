package buildinfo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The release's scripts and workflows (00-M12 4.10, M12/P5/S1): the three workflows are read as data; the gate and the
// notes run against a fake gh and throwaway repositories, executed directly so that their executable bit and #! line
// count. They need bash, git, jq and awk, which README's "需要安装" lists.

// repoRoot is the repository's root, absolute.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// needTools fails the test when one of the tools the release's scripts run is missing: they are on README's list.
func needTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"bash", "git", "jq", "awk"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the release scripts' tests need %s, on README's list of what to install (需要安装): %v", tool, err)
		}
	}
}

// scriptEnv is a script's whole environment: bin (a fake's directory) first in PATH, then the real PATH; a HOME of
// its own; no git configuration of the user's (signing, say) but an author; C.UTF-8; and extra, "KEY=value" each.
func scriptEnv(t *testing.T, bin string, extra ...string) []string {
	t.Helper()
	home := t.TempDir()
	return append([]string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Release Test", "GIT_AUTHOR_EMAIL=release@example.test",
		"GIT_COMMITTER_NAME=Release Test", "GIT_COMMITTER_EMAIL=release@example.test",
		"LC_ALL=C.UTF-8",
	}, extra...)
}

// ran is what a script run wrote and how it exited.
type ran struct {
	stdout, stderr string
	code           int
}

// runScript executes the repository's deploy/<script> in dir with env, without bash in between.
func runScript(t *testing.T, dir string, env []string, script string, args ...string) ran {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRoot(t), "deploy", script), args...)
	cmd.Dir, cmd.Env = dir, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := ran{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		r.code = exit.ExitCode()
	case err != nil:
		t.Fatalf("deploy/%s did not run: %v", script, err)
	}
	return r
}

// git runs git in dir with env, failing the test when it fails, and returns its output trimmed.
func git(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes content to path, with mode.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// fakeGH is the gh the gate calls: it logs each call to FAKE_GH_LOG and answers only api, finding the path (with
// its query) in FAKE_GH_ROUTES ("path<TAB>file") and printing the file; without a route it says 404 and exits 1,
// as gh does.
const fakeGH = `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_GH_LOG"
[ "$1" = api ] || exit 2
file=$(awk -F '\t' -v p="$2" '$1 == p { print $2 }' "$FAKE_GH_ROUTES")
[ -n "$file" ] || { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
cat "$file"
`

// workflowRun is a run as GitHub's REST API answers it, with the fields the gate reads and a few besides
// (m12/p5-design/ci-runs.json, the API's answer for 86565dd6).
type workflowRun struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	DisplayTitle string  `json:"display_title"`
	HeadSHA      string  `json:"head_sha"`
	HeadBranch   string  `json:"head_branch"`
	Event        string  `json:"event"`
	Status       string  `json:"status"`
	Conclusion   *string `json:"conclusion"`
	RunAttempt   int     `json:"run_attempt"`
	RunNumber    int     `json:"run_number"`
	WorkflowID   int64   `json:"workflow_id"`
	CreatedAt    string  `json:"created_at"`
}

const (
	ciPath      = ".github/workflows/ci.yml"
	releasePath = ".github/workflows/release.yml"
)

// gateRun is a push run of workflow on main for sha: status, and conclusion ("" for null).
func gateRun(id int64, path, sha, status, conclusion string, attempt int, created string) workflowRun {
	r := workflowRun{ID: id, Path: path, DisplayTitle: "Merge branch 'm12-release'", HeadSHA: sha, HeadBranch: "main",
		Event: "push", Status: status, RunAttempt: attempt, RunNumber: int(id % 1000), CreatedAt: created}
	r.Name, r.WorkflowID = "CI", 370893547
	if path == releasePath {
		r.Name, r.WorkflowID = "Release", 370893999
	}
	if conclusion != "" {
		r.Conclusion = &conclusion
	}
	return r
}

// answer is the API's answer of runs, in their order: the API answers newest first.
func answer(runs ...workflowRun) string {
	b, err := json.Marshal(map[string]any{"total_count": len(runs), "workflow_runs": append([]workflowRun{}, runs...)})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// gateRepo is a throwaway clone with an origin: main's one commit pushed and fetched, and a commit on a side
// branch only.
type gateRepo struct {
	dir, main, side string
	env             []string
	bin             string
}

func newGateRepo(t *testing.T) gateRepo {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bin, "gh"), fakeGH, 0o755)
	env := scriptEnv(t, bin)
	git(t, root, env, "init", "-q", "--bare", "-b", "main", "origin.git")
	dir := filepath.Join(root, "clone")
	git(t, root, env, "init", "-q", "-b", "main", dir)
	git(t, dir, env, "remote", "add", "origin", filepath.Join(root, "origin.git"))
	writeFile(t, filepath.Join(dir, "a"), "a\n", 0o644)
	git(t, dir, env, "add", "a")
	git(t, dir, env, "commit", "-q", "-m", "one")
	git(t, dir, env, "push", "-q", "origin", "HEAD:main")
	git(t, dir, env, "fetch", "-q", "origin")
	main := git(t, dir, env, "rev-parse", "HEAD")
	git(t, dir, env, "checkout", "-q", "-b", "side")
	writeFile(t, filepath.Join(dir, "b"), "b\n", 0o644)
	git(t, dir, env, "add", "b")
	git(t, dir, env, "commit", "-q", "-m", "side")
	side := git(t, dir, env, "rev-parse", "HEAD")
	git(t, dir, env, "checkout", "-q", "main")
	return gateRepo{dir: dir, main: main, side: side, env: env, bin: bin}
}

// The changelogs of the gate's cases.
const (
	unreleased = "# 变更记录\n\n## 0.1.0（未发布）\n\n- 第一个版本。\n\n## 0.0.9（2026-01-01）\n\n- 更早的。\n"
	dated      = "# 变更记录\n\n## 0.1.0（2026-11-20）\n\n- 第一个版本。\n"
)

// The release gate (00-M12 4.10, M12/P5/S1, P5 design 3.6 and 3.9's table): each refusing case fails one check
// alone, the rest as in "an rc passes", and wants exit 1, nothing on stdout and the words of that check on stderr,
// not exit 1 alone: an earlier check that exits first would hide a later one.
func TestTheReleaseGate(t *testing.T) {
	needTools(t)
	type gateCase struct {
		name, tag, changelog string
		// ci is the answer of ci.yml's runs for the tagged commit; with noRoutes the fake gh answers nothing
		// and fails
		ci           func(sha string) string
		noRoutes     bool
		onSide       bool   // the tag names the side branch's commit
		githubSHA    string // "main" (the default), "side" or "none"
		image        string // "" is registry.test/nw-gate; "-" leaves IMAGE out
		wantCode     int
		wantStderr   []string
		wantNoGH     bool // the fake gh is never called
		wantVersion  string
		wantPrerel   string
		checkOutputs bool
	}
	green := func(sha string) string {
		return answer(gateRun(43745380462, ciPath, sha, "completed", "success", 1, "2026-11-20T01:00:00Z"))
	}
	cases := []gateCase{
		{name: "an rc passes", tag: "v0.1.0-rc.1", changelog: unreleased, ci: green, wantVersion: "0.1.0-rc.1",
			wantPrerel: "true", checkOutputs: true},
		{name: "an rc passes without GITHUB_SHA, as run by hand", tag: "v0.1.0-rc.1", changelog: unreleased, ci: green,
			githubSHA: "none", wantVersion: "0.1.0-rc.1", wantPrerel: "true"},
		{name: "a release passes with a dated heading", tag: "v0.1.0", changelog: dated, ci: green,
			wantVersion: "0.1.0", wantPrerel: "false"},
		{name: "a release whose heading has no date", tag: "v0.1.0", changelog: unreleased, ci: green, wantCode: 1,
			wantStderr: []string{"要写发布的日期"}, wantNoGH: true},
		{name: "no section of 0.1.0", tag: "v0.1.0-rc.1", changelog: "# 变更记录\n\n## 0.0.9（2026-01-01）\n\n- 更早的。\n",
			ci: green, wantCode: 1, wantStderr: []string{"的标题有 0 个"}, wantNoGH: true},
		{name: "two sections of 0.1.0", tag: "v0.1.0-rc.1", changelog: unreleased + "\n## 0.1.0（2026-11-20）\n\n- 又一节。\n",
			ci: green, wantCode: 1, wantStderr: []string{"的标题有 2 个"}, wantNoGH: true},
		{name: "headings that only begin like it", tag: "v0.1.0-rc.1",
			changelog: "# 变更记录\n\n## 0.1.00（2026-11-20）\n\n- 不是。\n\n## 0.1.0-rc.1（未发布）\n\n- 也不是。\n",
			ci:        green, wantCode: 1, wantStderr: []string{"的标题有 0 个"}, wantNoGH: true},
		{name: "a dot is not a wildcard", tag: "v0.1.0-rc.1", changelog: "# 变更记录\n\n## 0x1x0（未发布）\n\n- 不是。\n",
			ci: green, wantCode: 1, wantStderr: []string{"的标题有 0 个"}, wantNoGH: true},
		{name: "an empty section before the next", tag: "v0.1.0-rc.1",
			changelog: "# 变更记录\n\n## 0.1.0（未发布）\n\n## 0.0.9（2026-01-01）\n\n- 更早的。\n",
			ci:        green, wantCode: 1, wantStderr: []string{"取不出或是空的", "是空的"}, wantNoGH: true},
		{name: "a dated section of blank lines alone", tag: "v0.1.0", changelog: "# 变更记录\n\n## 0.1.0（2026-11-20）\n\n\n  \n",
			ci: green, wantCode: 1, wantStderr: []string{"取不出或是空的", "是空的"}, wantNoGH: true},
		{name: "the tag is on a side branch", tag: "v0.1.0-rc.1", changelog: unreleased, ci: green, onSide: true,
			githubSHA: "side", wantCode: 1, wantStderr: []string{"不在 origin/main 上"}, wantNoGH: true},
		{name: "GITHUB_SHA is another commit", tag: "v0.1.0-rc.1", changelog: unreleased, ci: green, githubSHA: "side",
			wantCode: 1, wantStderr: []string{"触发这次运行的是"}, wantNoGH: true},
		{name: "ci.yml failed while the rehearsal passed", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				return answer(gateRun(43745380462, ciPath, sha, "completed", "failure", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"completed failure", "43745380462"}},
		{name: "ci.yml is still running", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				return answer(gateRun(43745380462, ciPath, sha, "in_progress", "", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"in_progress null"}},
		{name: "ci.yml passed on its second attempt", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				return answer(gateRun(43745380462, ciPath, sha, "completed", "success", 2, "2026-11-20T01:00:00Z"))
			},
			wantVersion: "0.1.0-rc.1", wantPrerel: "true"},
		{name: "two runs of ci.yml, the later failed", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				return answer(gateRun(43745380999, ciPath, sha, "completed", "failure", 1, "2026-11-20T02:00:00Z"),
					gateRun(43745380462, ciPath, sha, "completed", "success", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"completed failure", "43745380999"}},
		{name: "no run of ci.yml", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci:       func(string) string { return answer() },
			wantCode: 1, wantStderr: []string{"单独推上 main"}},
		{name: "gh fails", tag: "v0.1.0-rc.1", changelog: unreleased, noRoutes: true,
			wantCode: 1, wantStderr: []string{"读不到 ci.yml"}},
		{name: "the endpoint answers another workflow's run", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				return answer(gateRun(43745381111, releasePath, sha, "completed", "success", 1, "2026-11-20T01:00:05Z"))
			},
			wantCode: 1, wantStderr: []string{"单独推上 main"}},
		// the gate's own words: without its check the notes would refuse too, later and in theirs
		{name: "no IMAGE", tag: "v0.1.0-rc.1", changelog: unreleased, ci: green, image: "-",
			wantCode: 1, wantStderr: []string{"release-gate: 要 IMAGE"}, wantNoGH: true},
		{name: "a heading without its closing parenthesis", tag: "v0.1.0-rc.1",
			changelog: "# 变更记录\n\n## 0.1.0（未发布\n\n- 第一个版本。\n", ci: green, wantCode: 1,
			wantStderr: []string{"的标题有 0 个"}, wantNoGH: true},
		// the endpoint answers ci.yml's push runs on main of the commit; the gate checks each of the three again,
		// so that a newer green run of another branch, event or commit does not pass for ours, which failed
		{name: "a newer green run of another branch", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				other := gateRun(43745380999, ciPath, sha, "completed", "success", 1, "2026-11-20T02:00:00Z")
				other.HeadBranch = "m12-release"
				return answer(other, gateRun(43745380462, ciPath, sha, "completed", "failure", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"completed failure", "43745380462"}},
		{name: "a newer green run of a pull request", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				other := gateRun(43745380999, ciPath, sha, "completed", "success", 1, "2026-11-20T02:00:00Z")
				other.Event = "pull_request"
				return answer(other, gateRun(43745380462, ciPath, sha, "completed", "failure", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"completed failure", "43745380462"}},
		{name: "a newer green run of another commit", tag: "v0.1.0-rc.1", changelog: unreleased,
			ci: func(sha string) string {
				other := gateRun(43745380999, ciPath, strings.Repeat("e", 40), "completed", "success", 1, "2026-11-20T02:00:00Z")
				return answer(other, gateRun(43745380462, ciPath, sha, "completed", "failure", 1, "2026-11-20T01:00:00Z"))
			},
			wantCode: 1, wantStderr: []string{"completed failure", "43745380462"}},
	}
	for _, bad := range []string{"v0.1", "v0.1.0-beta.1", "v01.0.0", "v0.1.0-rc.0", "0.1.0"} {
		cases = append(cases, gateCase{name: "the tag " + bad, tag: bad, changelog: unreleased, ci: green, wantCode: 1,
			wantStderr: []string{"不是 vX.Y.Z"}, wantNoGH: true})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newGateRepo(t)
			tagged := r.main
			if c.onSide {
				tagged = r.side
			}
			git(t, r.dir, r.env, "tag", "-a", c.tag, "-m", "Nerve Wiki "+c.tag, tagged)
			// as actions/checkout leaves the gate job: GITHUB_SHA's commit, detached, or the tagged commit where
			// GITHUB_SHA is not set (the gate run by hand), so that a gate that asks about HEAD, in place of origin/main
			// or of the tag's commit, is caught where HEAD is not what it should have asked about
			head := tagged
			if c.githubSHA == "side" {
				head = r.side
			}
			git(t, r.dir, r.env, "checkout", "-q", "--detach", head)
			writeFile(t, filepath.Join(r.dir, "CHANGELOG.md"), c.changelog, 0o644)
			work := filepath.Dir(r.dir)
			log, routes, output := filepath.Join(work, "gh.log"), filepath.Join(work, "routes"), filepath.Join(work, "output")
			writeFile(t, log, "", 0o644)
			writeFile(t, output, "", 0o644)
			ciURL := "repos/o/r/actions/workflows/ci.yml/runs?head_sha=" + tagged + "&branch=main&event=push&per_page=100"
			allURL := "repos/o/r/actions/runs?head_sha=" + tagged + "&per_page=100"
			// every workflow's runs: the rehearsal of release.yml, later than ci.yml's run and green, before it
			all := answer(gateRun(43745381111, releasePath, tagged, "completed", "success", 1, "2026-11-20T01:00:05Z"),
				gateRun(43745380462, ciPath, tagged, "completed", "failure", 1, "2026-11-20T01:00:00Z"))
			writeFile(t, filepath.Join(work, "all.json"), all, 0o644)
			routeLines := ""
			if !c.noRoutes {
				writeFile(t, filepath.Join(work, "ci.json"), c.ci(tagged), 0o644)
				routeLines = ciURL + "\t" + filepath.Join(work, "ci.json") + "\n" + allURL + "\t" + filepath.Join(work, "all.json") + "\n"
			}
			writeFile(t, routes, routeLines, 0o644)
			env := append(append([]string{}, r.env...), "FAKE_GH_LOG="+log, "FAKE_GH_ROUTES="+routes,
				"GITHUB_REPOSITORY=o/r", "GITHUB_OUTPUT="+output)
			switch c.image {
			case "":
				env = append(env, "IMAGE=registry.test/nw-gate")
			case "-":
			default:
				env = append(env, "IMAGE="+c.image)
			}
			switch c.githubSHA {
			case "", "main":
				env = append(env, "GITHUB_SHA="+tagged)
			case "side":
				env = append(env, "GITHUB_SHA="+r.side)
			}
			got := runScript(t, r.dir, env, "release-gate.sh", c.tag)
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if got.code != c.wantCode {
				t.Fatalf("exit %d, want %d; stdout %q, stderr %q", got.code, c.wantCode, got.stdout, got.stderr)
			}
			if c.wantCode != 0 {
				if got.stdout != "" {
					t.Errorf("a refusal wrote %q to stdout, want nothing", got.stdout)
				}
				for _, words := range c.wantStderr {
					if !strings.Contains(got.stderr, words) {
						t.Errorf("stderr %q, want it to say %q", got.stderr, words)
					}
				}
				if c.wantNoGH && len(calls) != 0 {
					t.Errorf("the fake gh was called before the gate refused: %q", calls)
				}
				return
			}
			want := fmt.Sprintf("version=%s\nprerelease=%s\nsha=%s\n", c.wantVersion, c.wantPrerel, tagged)
			if got.stdout != want {
				t.Errorf("stdout %q, want %q (stderr %q)", got.stdout, want, got.stderr)
			}
			if !c.checkOutputs {
				return
			}
			if written, err := os.ReadFile(output); err != nil || string(written) != want {
				t.Errorf("GITHUB_OUTPUT holds %q (%v), want %q", written, err, want)
			}
			if string(calls) != "api "+ciURL+"\n" {
				t.Errorf("the fake gh was called as %q, want once, for ci.yml's runs: api %s", calls, ciURL)
			}
		})
	}
}

// notesChangelog has 0.1.0's section between blank lines and a line of spaces, with a heading of the third level,
// and 0.0.9's after it; a line before the section ends with a heading of the version, not at the line's start.
const notesChangelog = `# 变更记录

旧的一节并进了下面：### 0.1.0（草稿）
每个版本一节。

## 0.1.0（未发布）


第一个版本。

### 已知限制

- 一条。

` + "  \n" + `
## 0.0.9（2026-01-01）

- 不该出现。
`

// notesTail is the notes' generated part for version and tag, of registry.test/nw-notes and o2/r2, with ' for
// each backtick (a raw string holds none).
const notesTail = `---

## 镜像

'linux/amd64' 与 'linux/arm64' 两种架构，只有确切的版本号，没有 'latest'：

'''bash
docker pull registry.test/nw-notes:{version}
'''

索引的摘要：'sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'。部署、升级与备份照 [deploy/README.md](https://github.com/o2/r2/blob/{tag}/deploy/README.md)。

## 校验

来源证明（GitHub 的 artifact attestation，'gh' 要先 'gh auth login'）与签名（Sigstore，keyless，用 cosign v3；流水线用 v3.1.4）：

'''bash
gh attestation verify oci://registry.test/nw-notes:{version} -R o2/r2 \
  --cert-identity https://github.com/o2/r2/.github/workflows/release.yml@refs/tags/{tag}
cosign verify registry.test/nw-notes:{version} \
  --certificate-identity=https://github.com/o2/r2/.github/workflows/release.yml@refs/tags/{tag} \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
'''

两份 SBOM 的证明：第一条加 '--predicate-type https://spdx.dev/Document/v2.3'。

## 附件

- 'nervewiki-{version}-image.spdx.json'：镜像的 SBOM（SPDX 2.3，syft 从 amd64 的镜像生成，含二进制里的 Go 模块）；
- 'nervewiki-{version}-web.spdx.json'：内嵌的前端的生产依赖（SPDX 2.3）；
- 'nervewiki-{version}-licenses.tar.gz'：第三方许可证，与镜像里的 '/licenses' 相同。
`

// The release notes (00-M12 4.10, M12/P5/S1, P5 design 3.7): the version's section of CHANGELOG.md, an rc's with a
// sentence first, then the image, the index's digest, the two commands that verify it and the assets, compared
// whole. IMAGE and GITHUB_REPOSITORY are fakes unlike production and each other, so a name written in the script
// shows.
func TestTheReleaseNotes(t *testing.T) {
	needTools(t)
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	section := "第一个版本。\n\n### 已知限制\n\n- 一条。\n\n"
	tail := func(version, tag string) string {
		return strings.NewReplacer("'", "`", "{version}", version, "{tag}", tag).Replace(notesTail)
	}
	notes := func(t *testing.T, changelog string, extra []string, args ...string) ran {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "CHANGELOG.md"), changelog, 0o644)
		env := scriptEnv(t, t.TempDir(), append([]string{"IMAGE=registry.test/nw-notes", "GITHUB_REPOSITORY=o2/r2"}, extra...)...)
		return runScript(t, dir, env, "release-notes.sh", args...)
	}
	// a heading of the version the gate does not count (no closing parenthesis) is not the section's start: after
	// the release's heading it ends the section as the next version's would, before it the section starts later
	const broken = "## 0.1.0（草稿\n\n- 草稿里的一条。\n"
	const release = "## 0.1.0（2026-11-20）\n\n- 新的。\n"
	for _, c := range []struct{ name, changelog, tag, want string }{
		{"an rc", notesChangelog, "v0.1.0-rc.1",
			"这是预发布版本 v0.1.0-rc.1，供核对与试用；正式版本是 v0.1.0。\n\n" + section + tail("0.1.0-rc.1", "v0.1.0-rc.1")},
		{"a release", notesChangelog, "v0.1.0", section + tail("0.1.0", "v0.1.0")},
		{"a second heading of the version", "# 变更记录\n\n" + release + "\n" + broken, "v0.1.0", "- 新的。\n\n" + tail("0.1.0", "v0.1.0")},
		{"a broken heading of the version first", "# 变更记录\n\n" + broken + "\n" + release, "v0.1.0",
			"- 新的。\n\n" + tail("0.1.0", "v0.1.0")},
		// the gate refuses two whole headings of the version; the notes alone take the first section
		{"two headings of the version", "# 变更记录\n\n" + release + "\n## 0.1.0（2026-11-21）\n\n- 又一节。\n", "v0.1.0",
			"- 新的。\n\n" + tail("0.1.0", "v0.1.0")},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := notes(t, c.changelog, nil, c.tag, digest)
			if got.code != 0 || got.stdout != c.want {
				t.Errorf("exit %d, stderr %q, notes:\n%s\nwant:\n%s", got.code, got.stderr, got.stdout, c.want)
			}
		})
	}
	for _, c := range []struct {
		name, changelog string
		extra           []string
		args            []string
		words           string
	}{
		{"no section", "# 变更记录\n\n## 0.0.9（2026-01-01）\n\n- 更早的。\n", nil, []string{"v0.1.0-rc.1", digest}, "没有"},
		{"a broken heading alone", "# 变更记录\n\n" + broken, nil, []string{"v0.1.0", digest}, "没有"},
		{"a section of blank lines", "# 变更记录\n\n## 0.1.0（未发布）\n\n  \n\n## 0.0.9（2026-01-01）\n\n- 更早的。\n", nil,
			[]string{"v0.1.0", digest}, "是空的"},
		{"a digest that is not one", notesChangelog, nil, []string{"v0.1.0", "sha256:abc"}, "摘要"},
		{"a tag of another form", notesChangelog, nil, []string{"v0.1.0-beta.1", digest}, "不是 vX.Y.Z"},
		{"no IMAGE", notesChangelog, []string{"IMAGE="}, []string{"v0.1.0", digest}, "要 IMAGE"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := notes(t, c.changelog, c.extra, c.args...)
			if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, c.words) {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1, nothing and %q", got.code, got.stdout, got.stderr, c.words)
			}
		})
	}
}

// fakeDocker is the docker deploy/release-version-tag.sh calls. It logs each call to FAKE_DOCKER_LOG and answers only
// buildx imagetools inspect <ref> --format '{{json .Manifest}}' and buildx imagetools create -t <ref> <image>@<digest>.
// FAKE_DOCKER_STATE holds what the tag is: "absent" (inspect says not found, as docker does), "unknown" (it says
// manifest unknown, as a registry may), "error:<words>" (it fails with them) or a digest, which inspect answers as
// buildx does, the index's descriptor with its manifests. create writes its source's digest there, or
// FAKE_DOCKER_AFTER when that is set (a copy that came out as another index); it fails with FAKE_DOCKER_CREATE_FAILS
// set, or when the tag is there already: imagetools would move it, which the script must not ask for.
const fakeDocker = `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_DOCKER_LOG"
[ "$1 $2" = "buildx imagetools" ] || exit 2
state=$(cat "$FAKE_DOCKER_STATE")
case $3 in
inspect)
  [ $# -eq 6 ] && [ "$5" = --format ] && [ "$6" = '{{json .Manifest}}' ] || exit 2
  case $state in
  absent) echo "ERROR: $4: not found" >&2; exit 1 ;;
  unknown) echo "ERROR: $4: manifest unknown: manifest unknown" >&2; exit 1 ;;
  error:*) echo "ERROR: ${state#error:}" >&2; exit 1 ;;
  esac
  printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","digest":"%s","size":743,"manifests":[]}\n' "$state"
  ;;
create)
  [ $# -eq 6 ] && [ "$4" = -t ] || exit 2
  [ -z "${FAKE_DOCKER_CREATE_FAILS:-}" ] || { echo "ERROR: failed to push $5" >&2; exit 1; }
  case $state in sha256:*) echo "ERROR: $5 is there already" >&2; exit 1 ;; esac
  printf '%s\n' "${FAKE_DOCKER_AFTER:-${6#*@}}" >"$FAKE_DOCKER_STATE"
  ;;
*) exit 2 ;;
esac
`

// fakeGHRelease is the gh deploy/github-release.sh calls. It logs each call to FAKE_GH_LOG and answers only release
// view (--json isDraft --jq .isDraft, or --json assets --jq .assets[].name), create, edit and upload. FAKE_GH_STATE
// holds what the Release is: "absent" (view says release not found, as gh does), "draft", "published" or
// "error:<words>" (view fails with them); FAKE_GH_ASSETS its assets' names, a line each. create makes a draft and
// fails where there is a Release already; edit --draft=false publishes, unless FAKE_GH_EDIT_KEEPS is set; upload adds
// files that are there and fails on a published Release, as the repository's immutable releases would have it; edit
// and upload fail where there is no Release.
const fakeGHRelease = `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_GH_LOG"
[ "$1" = release ] || exit 2
state=$(cat "$FAKE_GH_STATE")
case $state in
error:*) echo "${state#error:}" >&2; exit 1 ;;
absent) [ "$2" = create ] || { echo "release not found" >&2; exit 1; } ;;
esac
case $2 in
view)
  case "${*:4}" in
  "--json isDraft --jq .isDraft") [ "$state" = draft ] && echo true || echo false ;;
  "--json assets --jq .assets[].name") cat "$FAKE_GH_ASSETS" ;;
  *) exit 2 ;;
  esac
  ;;
create)
  [ "$state" = absent ] || { echo "a release with the same tag name already exists: $3" >&2; exit 1; }
  echo draft >"$FAKE_GH_STATE"
  ;;
edit)
  case " $* " in
  *" --draft=false "*) [ -n "${FAKE_GH_EDIT_KEEPS:-}" ] || echo published >"$FAKE_GH_STATE" ;;
  esac
  ;;
upload)
  [ "$state" = draft ] || { echo "HTTP 422: Cannot upload assets to an immutable release." >&2; exit 1; }
  for f in "${@:4}"; do
    [ "$f" = --clobber ] && continue
    [ -s "$f" ] || { echo "$f: no such file" >&2; exit 1; }
    grep -qxF "${f##*/}" "$FAKE_GH_ASSETS" || printf '%s\n' "${f##*/}" >>"$FAKE_GH_ASSETS"
  done
  ;;
*) exit 2 ;;
esac
`

// fakeRun is a run of a release script against a fake: the script's exit and what it wrote, the fake's calls, the
// state the fake was left in and the directory the run had.
type fakeRun struct {
	ran
	calls      []string
	state, dir string
}

// runAgainstFake runs deploy/<script> with args in a directory of its own, the fake (its name and content) first in
// PATH, its log and its state file (holding state) in prefix_LOG and prefix_STATE, and the variables extra. files are
// written into the directory first, a path relative to it each. In args and extra, {dir} is the directory.
func runAgainstFake(t *testing.T, fake [2]string, prefix, state string, files map[string]string, extra []string,
	script string, args ...string) fakeRun {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bin, fake[0]), fake[1], 0o755)
	log, stateFile := filepath.Join(dir, "calls.log"), filepath.Join(dir, "state")
	writeFile(t, log, "", 0o644)
	writeFile(t, stateFile, state+"\n", 0o644)
	for path, content := range files {
		path = filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, content, 0o644)
	}
	inDir := strings.NewReplacer("{dir}", dir)
	env := []string{prefix + "_LOG=" + log, prefix + "_STATE=" + stateFile}
	for _, e := range extra {
		env = append(env, inDir.Replace(e))
	}
	args = slices.Clone(args)
	for i := range args {
		args[i] = inDir.Replace(args[i])
	}
	got := runScript(t, dir, scriptEnv(t, bin, env...), script, args...)
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	r := fakeRun{ran: got, state: strings.TrimSpace(string(left)), dir: dir}
	if text := strings.TrimSpace(string(calls)); text != "" {
		r.calls = strings.Split(text, "\n")
	}
	return r
}

// The version tag and the Release (00-M12 4.10, M12/P5/S2, P5 design 3.13): the two steps of a tag's run that a
// normal rc never takes but on an error or a rerun, each branch run against a fake docker or gh. A case wants the
// exit, the words of its outcome (stderr on a refusal, stdout otherwise), the fake's calls whole and in order, and
// the state it leaves, so that a check gone, or a call made where none should be, shows.
func TestTheReleaseScripts(t *testing.T) {
	needTools(t)
	t.Run("the version tag", func(t *testing.T) {
		const image = "registry.test/nw-tag"
		digest, other := "sha256:"+strings.Repeat("1a", 32), "sha256:"+strings.Repeat("2b", 32)
		inspect := "buildx imagetools inspect " + image + ":0.1.0 --format {{json .Manifest}}"
		create := "buildx imagetools create -t " + image + ":0.1.0 " + image + "@" + digest
		for _, c := range []struct {
			name, state string
			extra, args []string // args nil: the image, 0.1.0 and digest
			code        int
			words       string
			calls       []string
			left        string
		}{
			{name: "absent: copied from the index", state: "absent", words: image + ":0.1.0 → " + digest,
				calls: []string{inspect, create, inspect}, left: digest},
			{name: "manifest unknown: absent too", state: "unknown", words: image + ":0.1.0 → " + digest,
				calls: []string{inspect, create, inspect}, left: digest},
			{name: "there, the same digest: left as it is", state: digest, words: "已指向 " + digest + "，不动",
				calls: []string{inspect, inspect}, left: digest},
			{name: "there, another digest: not moved", state: other, code: 1, words: "已指向 " + other + "，不改指",
				calls: []string{inspect}, left: other},
			{name: "unauthorized: not taken for absent", state: "error:unauthorized: authentication required", code: 1,
				words: "读不到 " + image + ":0.1.0", calls: []string{inspect}, left: "error:unauthorized: authentication required"},
			{name: "the copy fails", state: "absent", extra: []string{"FAKE_DOCKER_CREATE_FAILS=1"}, code: 1,
				words: "复制不出", calls: []string{inspect, create}, left: "absent"},
			{name: "the copy is another index", state: "absent", extra: []string{"FAKE_DOCKER_AFTER=" + other}, code: 1,
				words: "是 " + other + "，要 " + digest, calls: []string{inspect, create, inspect}, left: other},
			{name: "a digest of another form", state: "absent", args: []string{image, "0.1.0", "sha256:abc"}, code: 1,
				words: "摘要 \"sha256:abc\" 不对", left: "absent"},
			{name: "a version of another form", state: "absent", args: []string{image, "v0.1.0", digest}, code: 1,
				words: "版本号 \"v0.1.0\" 不对", left: "absent"},
		} {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				args := c.args
				if args == nil {
					args = []string{image, "0.1.0", digest}
				}
				r := runAgainstFake(t, [2]string{"docker", fakeDocker}, "FAKE_DOCKER", c.state, nil, c.extra,
					"release-version-tag.sh", args...)
				checkFakeRun(t, r, c.code, c.words, c.calls, c.left)
			})
		}
	})
	t.Run("the Release", func(t *testing.T) {
		three := func(version string) map[string]string {
			return map[string]string{
				"out/nervewiki-" + version + "-image.spdx.json": "{}\n",
				"out/nervewiki-" + version + "-web.spdx.json":   "{}\n",
				"out/nervewiki-" + version + "-licenses.tar.gz": "tar\n",
				"notes.md": "说明\n",
				"assets":   "",
			}
		}
		names := func(version string) string {
			return "nervewiki-" + version + "-image.spdx.json\nnervewiki-" + version + "-web.spdx.json\nnervewiki-" +
				version + "-licenses.tar.gz\n"
		}
		// with returns files with the assets the Release has, and without paths
		with := func(files map[string]string, assets string, without ...string) map[string]string {
			files["assets"] = assets
			for _, path := range without {
				delete(files, path)
			}
			return files
		}
		// plus returns files with one more under path
		plus := func(files map[string]string, path string) map[string]string {
			files[path] = "{}\n"
			return files
		}
		const rc, final = "v0.1.0-rc.1", "v0.1.0"
		view := func(tag string) string { return "release view " + tag + " --json isDraft --jq .isDraft" }
		viewAssets := func(tag string) string { return "release view " + tag + " --json assets --jq .assets[].name" }
		upload := func(tag, version string) string {
			return "release upload " + tag + " {dir}/out/nervewiki-" + version + "-image.spdx.json {dir}/out/nervewiki-" +
				version + "-web.spdx.json {dir}/out/nervewiki-" + version + "-licenses.tar.gz --clobber"
		}
		draft := func(tag string) []string { return []string{"draft", tag, "{dir}/notes.md", "{dir}/out"} }
		for _, c := range []struct {
			name, state string
			files       map[string]string
			extra, args []string
			code        int
			words       string
			calls       []string
			left        string
			assets      string // the Release's assets after, unless unchanged
		}{
			{name: "draft, none yet, an rc", state: "absent", files: three("0.1.0-rc.1"), args: draft(rc),
				words: "草稿与三个附件就绪", calls: []string{view(rc),
					"release create " + rc + " --draft --verify-tag --title Nerve Wiki " + rc + " --notes-file {dir}/notes.md --prerelease",
					upload(rc, "0.1.0-rc.1"), viewAssets(rc)}, left: "draft", assets: names("0.1.0-rc.1")},
			{name: "draft, none yet, a release", state: "absent", files: three("0.1.0"), args: draft(final),
				words: "草稿与三个附件就绪", calls: []string{view(final),
					"release create " + final + " --draft --verify-tag --title Nerve Wiki " + final + " --notes-file {dir}/notes.md",
					upload(final, "0.1.0"), viewAssets(final)}, left: "draft", assets: names("0.1.0")},
			{name: "draft, a draft already", state: "draft", files: with(three("0.1.0-rc.1"), "nervewiki-0.1.0-rc.1-web.spdx.json\n"),
				args: draft(rc), words: "草稿与三个附件就绪", calls: []string{view(rc),
					"release edit " + rc + " --notes-file {dir}/notes.md --prerelease", upload(rc, "0.1.0-rc.1"), viewAssets(rc)},
				left: "draft", assets: "nervewiki-0.1.0-rc.1-web.spdx.json\nnervewiki-0.1.0-rc.1-image.spdx.json\nnervewiki-0.1.0-rc.1-licenses.tar.gz\n"},
			{name: "draft, a draft with an asset of another name", state: "draft",
				files: with(three("0.1.0-rc.1"), "nervewiki-0.1.0-rc.1-old.txt\n"), args: draft(rc), code: 1,
				words: "传过之后 " + rc + " 的附件不对", calls: []string{view(rc),
					"release edit " + rc + " --notes-file {dir}/notes.md --prerelease", upload(rc, "0.1.0-rc.1"), viewAssets(rc)},
				left: "draft", assets: "nervewiki-0.1.0-rc.1-old.txt\n" + names("0.1.0-rc.1")},
			{name: "draft, published already with its assets", state: "published", files: with(three("0.1.0"), names("0.1.0")),
				args: draft(final), words: final + " 已公开，附件齐，不动", calls: []string{view(final), viewAssets(final)},
				left: "published"},
			{name: "draft, published already short of an asset", state: "published",
				files: with(three("0.1.0"), "nervewiki-0.1.0-image.spdx.json\nnervewiki-0.1.0-web.spdx.json\n"), args: draft(final),
				code: 1, words: final + " 已公开，附件不对", calls: []string{view(final), viewAssets(final)}, left: "published"},
			{name: "draft, view answers HTTP 502", state: "error:HTTP 502: Bad Gateway (https://api.github.com/graphql)",
				files: three("0.1.0"), args: draft(final), code: 1, words: "读不到 Release " + final, calls: []string{view(final)},
				left: "error:HTTP 502: Bad Gateway (https://api.github.com/graphql)"},
			{name: "draft, an asset short", state: "absent", files: with(three("0.1.0"), "", "out/nervewiki-0.1.0-web.spdx.json"),
				args: draft(final), code: 1, words: "附件目录", left: "absent"},
			{name: "draft, an asset over", state: "absent",
				files: plus(three("0.1.0"), "out/nervewiki-0.1.0-web.spdx.json.bak"), args: draft(final), code: 1,
				words: "附件目录", left: "absent"},
			{name: "draft, no notes", state: "absent", files: with(three("0.1.0"), "", "notes.md"), args: draft(final), code: 1,
				words: "说明", left: "absent"},
			{name: "draft, a tag of another form", state: "absent", files: three("0.1.0"),
				args: []string{"draft", "v0.1", "{dir}/notes.md", "{dir}/out"}, code: 1, words: "不是 vX.Y.Z", left: "absent"},
			{name: "draft, no GH_REPO", state: "absent", files: three("0.1.0"), extra: []string{"GH_REPO="}, args: draft(final),
				code: 1, words: "要 GH_REPO", left: "absent"},
			{name: "publish, a draft with its assets", state: "draft", files: with(three("0.1.0"), names("0.1.0")),
				args: []string{"publish", final}, words: final + " 已公开", calls: []string{view(final), viewAssets(final),
					"release edit " + final + " --draft=false", view(final)}, left: "published"},
			{name: "publish, a draft short of an asset", state: "draft",
				files: with(three("0.1.0"), "nervewiki-0.1.0-image.spdx.json\nnervewiki-0.1.0-licenses.tar.gz\n"),
				args:  []string{"publish", final}, code: 1, words: "草稿附件不齐，不公开", calls: []string{view(final), viewAssets(final)},
				left: "draft"},
			{name: "publish, a draft that stays one", state: "draft", files: with(three("0.1.0"), names("0.1.0")),
				extra: []string{"FAKE_GH_EDIT_KEEPS=1"}, args: []string{"publish", final}, code: 1, words: "公开之后 " + final + " 是 draft",
				calls: []string{view(final), viewAssets(final), "release edit " + final + " --draft=false", view(final)}, left: "draft"},
			{name: "publish, published already", state: "published", files: with(three("0.1.0"), names("0.1.0")),
				args: []string{"publish", final}, words: final + " 已公开，附件齐，不动", calls: []string{view(final), viewAssets(final)},
				left: "published"},
			{name: "publish, published short of an asset", state: "published",
				files: with(three("0.1.0"), "nervewiki-0.1.0-image.spdx.json\n"), args: []string{"publish", final}, code: 1,
				words: final + " 已公开，附件不对", calls: []string{view(final), viewAssets(final)}, left: "published"},
			{name: "publish, none", state: "absent", files: three("0.1.0"), args: []string{"publish", final}, code: 1,
				words: "没有 " + final + " 的 Release", calls: []string{view(final)}, left: "absent"},
		} {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				extra := append([]string{"FAKE_GH_ASSETS={dir}/assets", "GH_REPO=o3/r3"}, c.extra...)
				r := runAgainstFake(t, [2]string{"gh", fakeGHRelease}, "FAKE_GH", c.state, c.files, extra,
					"github-release.sh", c.args...)
				calls := make([]string, len(c.calls))
				for i, call := range c.calls {
					calls[i] = strings.ReplaceAll(call, "{dir}", r.dir)
				}
				checkFakeRun(t, r, c.code, c.words, calls, c.left)
				before := c.files["assets"]
				if c.assets != "" {
					before = c.assets
				}
				if after, err := os.ReadFile(filepath.Join(r.dir, "assets")); err != nil || string(after) != before {
					t.Errorf("the Release's assets are %q (%v), want %q", after, err, before)
				}
			})
		}
	})
}

// checkFakeRun wants a run of a release script to exit with code, to say words (on stderr when it refuses, on stdout
// otherwise, nothing on stdout then), to have called the fake exactly as calls says and to leave it in state left.
func checkFakeRun(t *testing.T, r fakeRun, code int, words string, calls []string, left string) {
	t.Helper()
	if r.code != code {
		t.Errorf("exit %d, want %d; stdout %q, stderr %q", r.code, code, r.stdout, r.stderr)
	}
	said := r.stdout
	if code != 0 {
		said = r.stderr
		if r.stdout != "" {
			t.Errorf("a refusal wrote %q to stdout, want nothing", r.stdout)
		}
	}
	if !strings.Contains(said, words) {
		t.Errorf("it said %q (stderr %q), want %q in it", said, r.stderr, words)
	}
	if !slices.Equal(r.calls, calls) {
		t.Errorf("the fake was called as\n\t%s\nwant\n\t%s", strings.Join(r.calls, "\n\t"), strings.Join(calls, "\n\t"))
	}
	if r.state != left {
		t.Errorf("the fake is left %q, want %q", r.state, left)
	}
}

// workflow is a workflow file decoded as YAML: "on" stays a string key.
type workflow = map[string]any

// readWorkflow decodes .github/workflows/<name>.
func readWorkflow(t *testing.T, name string) workflow {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var w workflow
	if err := yaml.Unmarshal(content, &w); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return w
}

// field is m[key] when m is a map, else nil.
func field(m any, key string) any {
	if mm, ok := m.(map[string]any); ok {
		return mm[key]
	}
	return nil
}

// jobsOf is a workflow's jobs by id.
func jobsOf(t *testing.T, name string, w workflow) map[string]any {
	t.Helper()
	jobs, ok := w["jobs"].(map[string]any)
	if !ok || len(jobs) == 0 {
		t.Fatalf("%s: no jobs", name)
	}
	return jobs
}

// stepsOf is a job's steps.
func stepsOf(job any) []map[string]any {
	var steps []map[string]any
	list, _ := field(job, "steps").([]any)
	for _, s := range list {
		if m, ok := s.(map[string]any); ok {
			steps = append(steps, m)
		}
	}
	return steps
}

// needsOf is a job's needs, one or a list.
func needsOf(job any) []string {
	switch n := field(job, "needs").(type) {
	case string:
		return []string{n}
	case []any:
		var needs []string
		for _, v := range n {
			s, _ := v.(string)
			needs = append(needs, s)
		}
		return needs
	}
	return nil
}

// stringsIn is every string in v, a decoded YAML value: its maps' keys and values and its lists' items, deep.
func stringsIn(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case map[string]any:
		var all []string
		for k, item := range v {
			all = append(append(all, k), stringsIn(item)...)
		}
		return all
	case []any:
		var all []string
		for _, item := range v {
			all = append(all, stringsIn(item)...)
		}
		return all
	}
	return nil
}

// yamlStrings is a YAML list of strings, as the decoder gives one.
func yamlStrings(v ...string) []any {
	list := make([]any, 0, len(v))
	for _, s := range v {
		list = append(list, s)
	}
	return list
}

// releaseJob is a row of the table of release.yml's jobs: when it runs, what its token may do, and the jobs it
// needs at least.
type releaseJob struct {
	when        string
	permissions map[string]any
	needs       []string
}

// releaseJobs is the table of release.yml's jobs (P5 design 3.9): a new job goes in here with its permissions before
// the workflow takes it. Only publish signs and writes GHCR's index and tags; draft and release write the Release.
func releaseJobs() map[string]releaseJob {
	const tag = "github.ref_type == 'tag'"
	return map[string]releaseJob{
		"rehearse": {"github.ref_type != 'tag'", map[string]any{"contents": "read"}, nil},
		"gate":     {tag, map[string]any{"contents": "read", "actions": "read"}, nil},
		"build":    {tag, map[string]any{"contents": "read", "packages": "write"}, []string{"gate"}},
		"sbom":     {tag, map[string]any{"contents": "read", "packages": "read"}, []string{"gate", "build"}},
		"publish": {tag, map[string]any{"contents": "read", "packages": "write", "id-token": "write",
			"attestations": "write", "artifact-metadata": "write"}, []string{"gate", "build", "sbom"}},
		"draft": {tag, map[string]any{"contents": "write"}, []string{"gate", "publish", "sbom"}},
		"verify": {tag, map[string]any{"contents": "read", "packages": "read", "attestations": "read"},
			[]string{"gate", "publish", "draft"}},
		"release": {tag, map[string]any{"contents": "write"}, []string{"gate", "verify", "draft"}},
	}
}

// stepsRunning are the 1-based numbers of a job's steps whose run is exactly run.
func stepsRunning(job any, run string) []int {
	var found []int
	for i, step := range stepsOf(job) {
		if step["run"] == run {
			found = append(found, i+1)
		}
	}
	return found
}

// matrixOnly is the one expression a run's text may hold, at a ${{: the matrix's, spaces or none around it.
var matrixOnly = regexp.MustCompile(`^\$\{\{\s*matrix\.[a-z_]+\s*\}\}`)

// expressionsBesidesTheMatrix are the expressions in a run's text that are not the matrix's, each from its ${{
// on (60 bytes at most). Each ${{ is checked where it starts, not by first cutting out a ${{ … }}: an expression
// may hold a "}" itself, as format('{0}', …) does, and a pattern of what lies between the braces would pass it.
func expressionsBesidesTheMatrix(run string) []string {
	var found []string
	for rest := run; ; rest = rest[3:] {
		at := strings.Index(rest, "${{")
		if at < 0 {
			return found
		}
		rest = rest[at:]
		if !matrixOnly.MatchString(rest) {
			found = append(found, rest[:min(60, len(rest))])
		}
	}
}

// Only a tag's jobs may write (00-M12 4.10, the hand-off M0-P6 item 1, M12/P5/S1): the three workflows read as
// data. ci.yml runs on branches alone, with a read-only token; vuln.yml reads only; release.yml's rehearsal reads
// only, and its tag's jobs have the permissions, the condition and the scripts of the table, with no way to go on
// after a failure. In all three a checkout leaves no token in .git (it goes into the image's build context), a
// run's text takes no expression but the matrix's, so that a tag's name or a commit's message reaches a script
// only through the environment, and an action is pinned by its commit.
func TestOnlyATagsJobsMayWrite(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"ci.yml", "release.yml", "vuln.yml"}; !slices.Equal(names, want) {
		t.Fatalf(".github/workflows holds %q, want %q: a new workflow's permissions are written here first", names, want)
	}
	readOnly := map[string]any{"contents": "read"}

	ci := readWorkflow(t, "ci.yml")
	if push := field(ci["on"], "push"); !reflect.DeepEqual(push, map[string]any{"branches": yamlStrings("**")}) {
		t.Errorf("ci.yml: on.push is %v, want branches [\"**\"] alone: a tag runs release.yml, not ci.yml again", push)
	}
	// actionlint runs nowhere else (decision 435): one step of the server job runs it, on the costs-1 leg, which is
	// one of the job's legs. The step is its name, its if and its run alone: a shell, an env (MAKEFLAGS=n), a
	// continue-on-error or a working directory of its own could let it pass without linting
	server := jobsOf(t, "ci.yml", ci)["server"]
	var legs []any
	if include, ok := field(field(field(server, "strategy"), "matrix"), "include").([]any); ok {
		for _, leg := range include {
			legs = append(legs, field(leg, "leg"))
		}
	}
	var lintActions []map[string]any
	for _, step := range stepsOf(server) {
		if run, _ := step["run"].(string); strings.Contains(run, "lint-actions") {
			lintActions = append(lintActions, step)
		}
	}
	lintStep := map[string]any{"name": "Lint workflows", "if": "matrix.leg == 'costs-1'", "run": "make lint-actions"}
	if len(lintActions) != 1 || !reflect.DeepEqual(lintActions[0], lintStep) || !slices.Contains(legs, any("costs-1")) {
		t.Errorf("ci.yml: the server job's steps that run lint-actions are %v and its legs %v, want one step %v "+
			"and nothing more, on a leg of the job: actionlint runs nowhere else", lintActions, legs, lintStep)
	}
	vuln := readWorkflow(t, "vuln.yml")
	if push, _ := field(vuln["on"], "push").(map[string]any); push["branches"] == nil || push["tags"] != nil {
		t.Errorf("vuln.yml: on.push is %v, want branches and no tags", push)
	}
	for name, w := range map[string]workflow{"ci.yml": ci, "vuln.yml": vuln} {
		if !reflect.DeepEqual(w["permissions"], readOnly) {
			t.Errorf("%s: permissions %v, want contents: read alone", name, w["permissions"])
		}
		for id, job := range jobsOf(t, name, w) {
			if p := field(job, "permissions"); p != nil {
				t.Errorf("%s: job %s has permissions %v of its own, want the workflow's", name, id, p)
			}
		}
	}

	release := readWorkflow(t, "release.yml")
	// an action or a reusable workflow by its commit's SHA, whose tag the comment beside it gives (13.5 item 2): a tag
	// or a branch moves under the workflow, with the token the job has (M12/P5/S2 fix round 1)
	pinned := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@\s]+)?@[0-9a-f]{40}$`)
	for name, w := range map[string]workflow{"ci.yml": ci, "vuln.yml": vuln, "release.yml": release} {
		for id, job := range jobsOf(t, name, w) {
			if uses, ok := field(job, "uses").(string); ok && !pinned.MatchString(uses) {
				t.Errorf("%s: job %s uses %q: by its commit's SHA, 40 hex digits", name, id, uses)
			}
			// the matrix is what a run may hold, so its values are written out: an expression there (a commit's
			// message as a value of the matrix) would reach the script as ${{ matrix.… }}
			for _, s := range stringsIn(field(job, "strategy")) {
				if strings.Contains(s, "${{") {
					t.Errorf("%s: job %s's strategy holds %q: the matrix's values are written out", name, id, s)
				}
			}
			for i, step := range stepsOf(job) {
				if uses, ok := step["uses"].(string); ok && !pinned.MatchString(uses) {
					t.Errorf("%s: job %s, step %d uses %q: by its commit's SHA, 40 hex digits", name, id, i+1, uses)
				}
				if uses, _ := step["uses"].(string); strings.HasPrefix(uses, "actions/checkout@") &&
					field(step["with"], "persist-credentials") != false {
					t.Errorf("%s: job %s, step %d checks out without persist-credentials: false", name, id, i+1)
				}
				run, _ := step["run"].(string)
				for _, e := range expressionsBesidesTheMatrix(run) {
					t.Errorf("%s: job %s, step %d runs %q: only the matrix may be written into a run, the rest "+
						"goes through env", name, id, i+1, e)
				}
			}
		}
	}

	on, _ := release["on"].(map[string]any)
	if want := map[string]any{"push": map[string]any{
		"branches": yamlStrings("**"), "tags": yamlStrings("v[0-9]+.[0-9]+.[0-9]+*"),
	}}; !reflect.DeepEqual(on, want) {
		t.Errorf("release.yml: on is %v, want %v alone", on, want)
	}
	if p, ok := release["permissions"].(map[string]any); !ok || len(p) != 0 {
		t.Errorf("release.yml: permissions %v, want {}", release["permissions"])
	}
	jobs, table := jobsOf(t, "release.yml", release), releaseJobs()
	for id := range jobs {
		if _, ok := table[id]; !ok {
			t.Errorf("release.yml: job %s is not in the table of this test", id)
		}
	}
	for id, want := range table {
		job, ok := jobs[id]
		if !ok {
			t.Errorf("release.yml: no job %s", id)
			continue
		}
		if when := field(job, "if"); when != want.when {
			t.Errorf("release.yml: job %s runs if %v, want %q and nothing more", id, when, want.when)
		}
		if p, ok := field(job, "permissions").(map[string]any); !ok || !reflect.DeepEqual(p, want.permissions) {
			t.Errorf("release.yml: job %s has permissions %v, want %v", id, field(job, "permissions"), want.permissions)
		}
		for _, need := range want.needs {
			if !slices.Contains(needsOf(job), need) {
				t.Errorf("release.yml: job %s needs %v, want %s among them", id, needsOf(job), need)
			}
		}
		if field(job, "continue-on-error") != nil {
			t.Errorf("release.yml: job %s has continue-on-error", id)
		}
		for i, step := range stepsOf(job) {
			if step["continue-on-error"] != nil {
				t.Errorf("release.yml: job %s, step %d has continue-on-error", id, i+1)
			}
			if (id == "gate" || id == "publish") && step["if"] != nil {
				t.Errorf("release.yml: %s's step %d runs if %v: each of its steps always runs", id, i+1, step["if"])
			}
		}
	}
	// the jobs that build or check an image smoke it in exactly one step. A step that smokes an image, and every step
	// of verify that runs commands (the checks before the Release is published), always runs and fails with them, in
	// Actions' bash -e: its keys are its name, env and run alone (no if to skip it, no shell without -e, no
	// continue-on-error), its run turns no option off with set +… (at a line's start, or after ;, &, |, { or ( ) or
	// shopt -o, and the job and the workflow give run no shell of their own; a smoke's line ends the command, nothing
	// after it to turn its failure into a success. One that could be skipped, or gone, lets an image or a Release
	// through unchecked, its job green (decisions 560, 562; M12/P5/S2 fix round 1). The ways on past a failure that
	// are written on purpose are not chased (decision 562): { …; } || true, BASH_ENV, a trap
	smokeLine := regexp.MustCompile(`(?m)^deploy/image-smoke\.sh [^|;&]*"\$VERSION"$`)
	optionOff := regexp.MustCompile(`(?m)(^|[;&|{(])\s*set\b[^\n]*\s\+[a-z]|\bshopt\b[^\n]*\s-[a-z]*o`)
	if shell := field(field(release["defaults"], "run"), "shell"); shell != nil {
		t.Errorf("release.yml: the workflow's defaults give run the shell %v: a smoke step runs in Actions' bash -e", shell)
	}
	for id, job := range jobs {
		var smokeSteps, failing []int
		for i, step := range stepsOf(job) {
			run, _ := step["run"].(string)
			smoke := strings.Contains(run, "image-smoke")
			if !smoke && (id != "verify" || run == "") {
				continue
			}
			failing = append(failing, i+1)
			for key, value := range step {
				if key != "name" && key != "env" && key != "run" {
					t.Errorf("release.yml: job %s, step %d (%v) has %s: %v besides its name, env and run: it always "+
						"runs and fails with its commands", id, i+1, step["name"], key, value)
				}
			}
			if off := optionOff.FindString(run); off != "" {
				t.Errorf("release.yml: job %s, step %d (%v) runs %q: a failure must fail the step", id, i+1, step["name"],
					strings.TrimSpace(off))
			}
			if !smoke {
				continue
			}
			smokeSteps = append(smokeSteps, i+1)
			if !smokeLine.MatchString(run) {
				t.Errorf("release.yml: job %s, step %d runs %q: want a line deploy/image-smoke.sh … \"$VERSION\" with "+
					"nothing after it", id, i+1, run)
			}
		}
		if len(failing) > 0 {
			if shell := field(field(field(job, "defaults"), "run"), "shell"); shell != nil {
				t.Errorf("release.yml: job %s smokes or checks in steps %v, and its defaults give run the shell %v", id,
					failing, shell)
			}
		}
		if (id == "rehearse" || id == "build" || id == "verify") && len(smokeSteps) != 1 {
			t.Errorf("release.yml: job %s smokes the image in steps %v, want one", id, smokeSteps)
		}
	}
	// the rehearsal reads only: its token's contents: read is all it has, with no secret (a token of someone's
	// would write past the job's permissions) and no login. The secrets context in any spelling (secrets.X,
	// secrets['X'], toJSON(secrets), SECRETS.X: contexts are not case-sensitive), in the rehearse job or in what
	// every job inherits, the workflow's env and defaults; a login by any action or command whose name holds it
	secretsContext := regexp.MustCompile(`(?i)\bsecrets\b`)
	for _, s := range stringsIn([]any{jobs["rehearse"], release["env"], release["defaults"]}) {
		if secretsContext.MatchString(s) {
			t.Errorf("release.yml: the rehearsal, or the env or defaults it inherits, names the secrets: %q", s)
		}
	}
	login := regexp.MustCompile(`(?i)login`)
	for i, step := range stepsOf(jobs["rehearse"]) {
		uses, _ := step["uses"].(string)
		run, _ := step["run"].(string)
		if login.MatchString(uses + "\n" + run) {
			t.Errorf("release.yml: the rehearsal's step %d logs in (%s%s): the rehearsal pushes nothing", i+1, uses, run)
		}
	}
	// the gate runs the script, then make vuln at high, one step each
	var gate, vulnStep []int
	for i, step := range stepsOf(jobs["gate"]) {
		switch step["run"] {
		case `deploy/release-gate.sh "$GITHUB_REF_NAME"`:
			gate = append(gate, i)
		case "make vuln VULN_AUDIT_LEVEL=high":
			vulnStep = append(vulnStep, i)
		}
	}
	if len(gate) != 1 || len(vulnStep) != 1 || gate[0] > vulnStep[0] {
		t.Errorf("release.yml: the gate's steps %v run the gate script and %v make vuln VULN_AUDIT_LEVEL=high, "+
			"want one each, the script first", gate, vulnStep)
	}
	// one BuildKit, by digest, for the rehearsal and the build
	buildkit := map[string]any{}
	for _, id := range []string{"rehearse", "build"} {
		for _, step := range stepsOf(jobs[id]) {
			if uses, _ := step["uses"].(string); strings.HasPrefix(uses, "docker/setup-buildx-action@") {
				buildkit[id] = field(step["with"], "driver-opts")
			}
		}
	}
	opts, _ := buildkit["rehearse"].(string)
	if !regexp.MustCompile(`^image=moby/buildkit:v[0-9.]+@sha256:[0-9a-f]{64}$`).MatchString(opts) ||
		buildkit["build"] != opts {
		t.Errorf("release.yml: the rehearsal's BuildKit is %v and the build's %v, want one moby/buildkit:<version>@sha256:…",
			buildkit["rehearse"], buildkit["build"])
	}
	// the steps of a tag's last jobs (P5 design 3.9 item 6, 3.13, 3.14): the version tag is publish's last step, after
	// the attestations and the signature, so that 0.1.0 is never there unsigned; the Release is drafted and published
	// by the script, which knows a draft from a published one; a third-party action in publish, which signs and writes
	// GHCR, is cosign-installer alone (decision 437), its login a shell's
	scripts := []struct{ job, run string }{
		{"publish", `deploy/release-version-tag.sh "$IMAGE" "$VERSION" "$DIGEST"`},
		{"draft", `deploy/github-release.sh draft "$GITHUB_REF_NAME" "$RUNNER_TEMP/notes.md" "$RUNNER_TEMP/out"`},
		{"release", `deploy/github-release.sh publish "$GITHUB_REF_NAME"`},
	}
	for _, want := range scripts {
		if got := stepsRunning(jobs[want.job], want.run); len(got) != 1 {
			t.Errorf("release.yml: job %s's steps %v run %s, want one", want.job, got, want.run)
		}
	}
	if steps := stepsOf(jobs["publish"]); len(steps) == 0 || steps[len(steps)-1]["run"] != scripts[0].run {
		t.Errorf("release.yml: publish's last step is not the version tag (%s): it comes after the signature", scripts[0].run)
	}
	for i, step := range stepsOf(jobs["publish"]) {
		if uses, _ := step["uses"].(string); uses != "" && !strings.HasPrefix(uses, "actions/") &&
			!strings.HasPrefix(uses, "sigstore/cosign-installer@") {
			t.Errorf("release.yml: publish's step %d uses %s: its third-party action is cosign-installer alone", i+1, uses)
		}
	}
	// before the version tag, publish signs the index in one step and attests it in three, the provenance and each
	// SBOM, pushed beside it; none of its steps has an if (above), so that none of them is skipped while the version
	// tag still comes (M12/P5/S2 fix round 1)
	index := "${{ steps.index.outputs.digest }}"
	sbomPaths := map[any]bool{}
	var signs, attests int
	for i, step := range stepsOf(jobs["publish"]) {
		if run, _ := step["run"].(string); strings.Contains(run, "cosign sign") {
			signs++
			if run != `cosign sign --yes "$IMAGE@$DIGEST"` || field(step["env"], "DIGEST") != index {
				t.Errorf("release.yml: publish's step %d runs %q with DIGEST %v, want cosign sign --yes \"$IMAGE@$DIGEST\" "+
					"of %s", i+1, run, field(step["env"], "DIGEST"), index)
			}
		}
		if uses, _ := step["uses"].(string); strings.HasPrefix(uses, "actions/attest@") {
			attests++
			with := step["with"]
			if field(with, "subject-digest") != index || field(with, "push-to-registry") != true {
				t.Errorf("release.yml: publish's step %d attests %v (push-to-registry %v), want %s pushed beside it", i+1,
					field(with, "subject-digest"), field(with, "push-to-registry"), index)
			}
			sbomPaths[field(with, "sbom-path")] = true
		}
	}
	wantPaths := map[any]bool{nil: true, "${{ runner.temp }}/out/nervewiki-${{ env.VERSION }}-image.spdx.json": true,
		"${{ runner.temp }}/out/nervewiki-${{ env.VERSION }}-web.spdx.json": true}
	if signs != 1 || attests != 3 || !reflect.DeepEqual(sbomPaths, wantPaths) {
		t.Errorf("release.yml: publish signs in %d steps and attests in %d, the SBOMs %v; want one and three, the "+
			"provenance, the image's SBOM and the web app's", signs, attests, sbomPaths)
	}
	// build builds the commit the gate let through, checked before the build, and publish's index is of images whose
	// revision is that commit: that the checkout of a tag's job is GITHUB_SHA's commit is an inference only a tag's run
	// proves (decision 561). The two steps are their keys alone, no env of their own to give SHA another value and no
	// if to skip them (M12/P5/S2 fix round 1)
	gateSHA := "${{ needs.gate.outputs.sha }}"
	keysOf := func(step map[string]any) []string {
		keys := make([]string, 0, len(step))
		for key := range step {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		return keys
	}
	headIsSHA, built := -1, -1
	for i, step := range stepsOf(jobs["build"]) {
		run, _ := step["run"].(string)
		if strings.HasPrefix(run, `test "$(git rev-parse HEAD)" = "$SHA" ||`) {
			headIsSHA = i
			if keys := keysOf(step); !slices.Equal(keys, []string{"name", "run"}) {
				t.Errorf("release.yml: build's step %d checks HEAD with the keys %v, want name and run alone", i+1, keys)
			}
		}
		if uses, _ := step["uses"].(string); strings.HasPrefix(uses, "docker/build-push-action@") {
			built = i
		}
	}
	if field(field(jobs["build"], "env"), "SHA") != gateSHA || headIsSHA < 0 || headIsSHA > built {
		t.Errorf("release.yml: build checks HEAD against SHA (%s) in step %d and builds in step %d, want the check "+
			"first, SHA %s", field(field(jobs["build"], "env"), "SHA"), headIsSHA+1, built+1, gateSHA)
	}
	var revision int
	for i, step := range stepsOf(jobs["publish"]) {
		if run, _ := step["run"].(string); strings.Contains(run, `[ "$(label revision)" = "$SHA" ] ||`) {
			revision++
			if keys := keysOf(step); !slices.Equal(keys, []string{"id", "name", "run"}) {
				t.Errorf("release.yml: publish's step %d checks the revision with the keys %v, want name, id and run "+
					"alone", i+1, keys)
			}
		}
	}
	if field(field(jobs["publish"], "env"), "SHA") != gateSHA || revision != 1 {
		t.Errorf("release.yml: publish checks the image's revision against SHA (%s) in %d steps, want one, SHA %s",
			field(field(jobs["publish"], "env"), "SHA"), revision, gateSHA)
	}
	// verify checks the attestations with the identity of the tag's release.yml, the provenance's and the SBOMs':
	// with -R alone, any workflow of the repository on any ref would do (P5 design 2.1 item 5)
	var verifies []string
	for _, step := range stepsOf(jobs["verify"]) {
		run, _ := step["run"].(string)
		for _, command := range strings.Split(strings.ReplaceAll(run, "\\\n", " "), "\n") {
			if strings.Contains(command, "gh attestation verify") {
				verifies = append(verifies, command)
			}
		}
	}
	var sboms int
	for _, command := range verifies {
		if !strings.Contains(command, `--cert-identity "$identity"`) {
			t.Errorf("release.yml: verify runs %q, without --cert-identity \"$identity\"", strings.TrimSpace(command))
		}
		if strings.Contains(command, "--predicate-type https://spdx.dev/Document/v2.3") {
			sboms++
		}
	}
	if len(verifies) < 2 || sboms == 0 || sboms == len(verifies) {
		t.Errorf("release.yml: verify checks the attestations with %q, want the provenance and the SBOMs "+
			"(--predicate-type https://spdx.dev/Document/v2.3)", verifies)
	}
	// the identity is the tag's release.yml word for word, in $identity and in the one cosign verify, with Actions'
	// issuer, nowhere a pattern; and verify runs on both architectures, as build (M12/P5/S2 fix round 1)
	const identity = "https://github.com/$GITHUB_REPOSITORY/.github/workflows/release.yml@$GITHUB_REF"
	identityLine := regexp.MustCompile(`(?m)^\s*identity="` + regexp.QuoteMeta(identity) + `"$`)
	var identities, cosigns int
	for i, step := range stepsOf(jobs["verify"]) {
		run, _ := step["run"].(string)
		if identityLine.MatchString(run) {
			identities++
		}
		if !strings.Contains(run, "cosign verify") {
			continue
		}
		cosigns++
		joined := strings.ReplaceAll(run, "\\\n", " ")
		for _, want := range []string{`--certificate-identity "` + identity + `"`,
			"--certificate-oidc-issuer https://token.actions.githubusercontent.com"} {
			if !strings.Contains(joined, want) {
				t.Errorf("release.yml: verify's step %d runs cosign verify without %s", i+1, want)
			}
		}
	}
	if identities != 1 || cosigns != 1 {
		t.Errorf("release.yml: verify sets identity=\"%s\" in %d steps and runs cosign verify in %d, want one each",
			identity, identities, cosigns)
	}
	for _, s := range stringsIn(jobs["verify"]) {
		if strings.Contains(s, "-regex") {
			t.Errorf("release.yml: verify holds %q: the identity is matched word for word", s)
		}
	}
	archs := func(job any) []any {
		var list []any
		include, _ := field(field(field(job, "strategy"), "matrix"), "include").([]any)
		for _, leg := range include {
			list = append(list, field(leg, "arch"))
		}
		return list
	}
	if got, want := archs(jobs["verify"]), yamlStrings("amd64", "arm64"); !reflect.DeepEqual(got, want) ||
		!reflect.DeepEqual(field(jobs["verify"], "strategy"), field(jobs["build"], "strategy")) {
		t.Errorf("release.yml: verify runs on %v, want %v, its strategy build's", got, want)
	}
	// the Release is published by release alone, after verify: no other job's step publishes it or calls gh release
	for id, job := range jobs {
		if id == "release" {
			continue
		}
		for i, step := range stepsOf(job) {
			run, _ := step["run"].(string)
			for _, word := range []string{"github-release.sh publish", "--draft=false", "gh release"} {
				if strings.Contains(run, word) {
					t.Errorf("release.yml: job %s, step %d runs %q: only release publishes the Release", id, i+1, word)
				}
			}
		}
	}
}
