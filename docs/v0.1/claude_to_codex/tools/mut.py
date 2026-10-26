# Negative controls (mutants): each mutant edits one file under server/, runs
# a command in server/, expects it to FAIL, and restores the file from a copy.
#
#   python3 mut.py spec.json [name ...]
#
# spec.json: [{"name", "file", "old", "new", "cmd", "timeout"?, "also"?}]
#   file    relative to server/ (or to $NW_SERVER, the server dir of another
#           checkout or worktree)
#   old/new the text replaced; old must occur exactly once
#   also    further [[old, new], ...] edits of the same file, each found once
#   cmd     a shell command run in server/; don't pipe it (its exit code counts)
#
# Results: FAILED(good) is a caught mutant. PASSED(BAD) survived. BUILD-ERROR,
# CMD-ERROR (the command did not run) and FAILED(unnamed) (failed without
# naming a test) are not caught until checked by hand. Never run make gen*
# while a mutant is applied, and never restore with git checkout --.
import json, os, shutil, subprocess, sys

here = os.path.dirname(os.path.abspath(__file__))
root = os.environ.get('NW_SERVER') or os.path.normpath(os.path.join(here, '..', '..', '..', '..', 'server'))
root = root.rstrip('/') + '/'
spec = json.load(open(sys.argv[1]))
only = set(sys.argv[2:])
results = []
for m in spec:
    if only and m['name'] not in only:
        continue
    path = root + m['file']
    orig = open(path).read()
    if orig.count(m['old']) != 1:
        print(f"{m['name']}: old text found {orig.count(m['old'])} times")
        results.append((m['name'], 'BADSPEC'))
        continue
    bak = path + '.mutbak'
    shutil.copyfile(path, bak)
    try:
        text = orig.replace(m['old'], m['new'])
        for old, new in m.get('also', []):
            if text.count(old) != 1:
                raise SystemExit(f"{m['name']}: also text found {text.count(old)} times")
            text = text.replace(old, new)
        open(path, 'w').write(text)
        try:
            p = subprocess.run(m['cmd'], shell=True, cwd=root, capture_output=True, text=True,
                               errors='replace', stdin=subprocess.DEVNULL, timeout=m.get('timeout', 600))
        except subprocess.TimeoutExpired:
            print(f"{m['name']}: TIMEOUT", flush=True)
            results.append((m['name'], 'TIMEOUT'))
            continue
        out = p.stdout + p.stderr
        status = 'FAILED(good)' if p.returncode != 0 else 'PASSED(BAD)'
        if 'build failed' in out or '[build failed]' in out or 'setup failed' in out:
            status = 'BUILD-ERROR'
        elif p.returncode in (126, 127) or 'command not found' in out:
            status = 'CMD-ERROR'
        elif p.returncode != 0 and not any(k in out for k in ('--- FAIL', 'FAIL\t', 'panic:', ' failed', 'FAIL ')):
            status = 'FAILED(unnamed)'
        lines = [l for l in out.splitlines() if 'FAIL' in l][:4]
        print(f"{m['name']}: {status}  {' | '.join(lines)[:300]}", flush=True)
        results.append((m['name'], status))
    finally:
        shutil.copyfile(bak, path)
        os.remove(bak)
bad = [r for r in results if r[1] != 'FAILED(good)']
print('ALL GOOD' if not bad else f'BAD: {bad}')
