# -*- coding: utf-8 -*-
"""Checks the relative links of every docs/ Markdown file that differs from a
base commit, and lists only the links that are broken now and were not broken
at the base (some quoted texts carry old broken links on purpose).

    python3 check_links.py [base]      # base defaults to origin/main

Links inside code spans are skipped; anchors are not checked. Read-only.
"""
import pathlib, re, subprocess, sys

ROOT = pathlib.Path(__file__).resolve().parents[4]
BASE = sys.argv[1] if len(sys.argv) > 1 else 'origin/main'
LINK = re.compile(r'\]\(([^)\s]+)\)')
CODE = re.compile(r'`[^`\n]*`')


def base_text(rel):
    r = subprocess.run(['git', '-C', str(ROOT), 'show', f'{BASE}:{rel}'], capture_output=True)
    return r.stdout if r.returncode == 0 else None


def broken_of(p, text):
    out = []
    for n, line in enumerate(text.split('\n'), 1):
        for m in LINK.finditer(CODE.sub('', line)):
            target = m.group(1).split('#')[0]
            if not target or re.match(r'[a-z]+:', target):
                continue
            if not (p.parent / target).resolve().exists():
                out.append((n, m.group(1)))
    return out


changed = broken = old = 0
for p in sorted((ROOT / 'docs').rglob('*.md')):
    rel = p.relative_to(ROOT).as_posix()
    base = base_text(rel)
    if p.read_bytes() == base:
        continue
    changed += 1
    was = [t for _, t in broken_of(p, base.decode('utf-8'))] if base else []
    for n, t in broken_of(p, p.read_text(encoding='utf-8')):
        if t in was:
            was.remove(t)
            old += 1
            continue
        broken += 1
        print(f'{rel}:{n}: {t}')
print(f'{changed} changed files, {broken} new broken links ({old} broken at {BASE} already)')
sys.exit(1 if broken else 0)
