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

// releaseJobs is the table of release.yml's jobs (P5 design 3.9). A job of S2 goes in here with its permissions
// before the workflow takes it.
func releaseJobs() map[string]releaseJob {
	return map[string]releaseJob{
		"rehearse": {"github.ref_type != 'tag'", map[string]any{"contents": "read"}, nil},
		"gate":     {"github.ref_type == 'tag'", map[string]any{"contents": "read", "actions": "read"}, nil},
		"build":    {"github.ref_type == 'tag'", map[string]any{"contents": "read", "packages": "write"}, []string{"gate"}},
	}
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
// after a failure. In all three a checkout leaves no token in .git (it goes into the image's build context) and a
// run's text takes no expression but the matrix's, so that a tag's name or a commit's message reaches a script
// only through the environment.
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
	for name, w := range map[string]workflow{"ci.yml": ci, "vuln.yml": vuln, "release.yml": release} {
		for id, job := range jobsOf(t, name, w) {
			// the matrix is what a run may hold, so its values are written out: an expression there (a commit's
			// message as a value of the matrix) would reach the script as ${{ matrix.… }}
			for _, s := range stringsIn(field(job, "strategy")) {
				if strings.Contains(s, "${{") {
					t.Errorf("%s: job %s's strategy holds %q: the matrix's values are written out", name, id, s)
				}
			}
			for i, step := range stepsOf(job) {
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
			if id == "gate" && step["if"] != nil {
				t.Errorf("release.yml: the gate's step %d runs if %v: each of its steps always runs", i+1, step["if"])
			}
		}
	}
	// the jobs that build an image smoke it in exactly one step (S2's verify joins them), and a step that smokes an
	// image always runs and fails with it: no if, and the script's line ends the command, nothing after it to turn
	// its failure into a success. One that could be skipped, or gone, lets an image through unsmoked, its job green
	smokeLine := regexp.MustCompile(`(?m)^deploy/image-smoke\.sh [^|;&]*"\$VERSION"$`)
	for id, job := range jobs {
		var smokes []int
		for i, step := range stepsOf(job) {
			run, _ := step["run"].(string)
			if !strings.Contains(run, "image-smoke") {
				continue
			}
			smokes = append(smokes, i+1)
			if step["if"] != nil {
				t.Errorf("release.yml: job %s, step %d smokes the image only if %v: it always runs", id, i+1, step["if"])
			}
			if !smokeLine.MatchString(run) {
				t.Errorf("release.yml: job %s, step %d runs %q: want a line deploy/image-smoke.sh … \"$VERSION\" with "+
					"nothing after it", id, i+1, run)
			}
		}
		if (id == "rehearse" || id == "build") && len(smokes) != 1 {
			t.Errorf("release.yml: job %s smokes the image in steps %v, want one", id, smokes)
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
}
