#!/usr/bin/env python3
"""Heuristic checker for the CLAUDE.md coding rules.

Hard-fails (exit 1) on security smells. Warns (and, with --strict, fails) on
comment bloat: high comment density and long comment runs — the things that
waste tokens and rot. Comment *quality* can't be fully mechanized, so treat
the comment report as "look here", not gospel.

Usage:
  scripts/check_rules.py            # report + fail on security only
  scripts/check_rules.py --strict   # also fail on comment bloat
"""
import os
import re
import sys

ROOTS = ["cmd", "internal", "web-ui/src"]
SKIP = ("internal/web/static/", "/node_modules/", "web-ui/dist/")
DENSITY_MAX = 0.30       # comment lines / code lines
RUN_MAX = 5              # consecutive comment-only lines

SECURITY = [  # hard-fail
    (re.compile(r'0o?777\b'), 'world-writable perms (0777)'),
    (re.compile(r'exec\.Command\(\s*"(sh|bash|zsh)"\s*,\s*"-c"\s*,\s*[^)]*\+'),
     'shell -c with string concatenation — likely unsanitized interpolation'),
]
REVIEW = [  # never fails; a "eyeball this" reminder
    (re.compile(r'exec\.Command\(\s*"(sh|bash|zsh)"\s*,\s*"-c"'),
     'shell -c pipeline — allowed only if inputs are BranchSafe-sanitized'),
]


def is_go(p): return p.endswith(".go")
def is_ts(p): return p.endswith((".ts", ".tsx"))


def walk():
    for root in ROOTS:
        for dp, _, files in os.walk(root):
            for f in files:
                p = os.path.join(dp, f)
                if any(s in "/" + p for s in SKIP):
                    continue
                if is_go(p) or is_ts(p):
                    yield p


def analyze(path):
    """Return (code_lines, comment_lines, longest_run, runs>=RUN_MAX)."""
    code = comment = run = longest = flagged = 0
    in_block = False
    with open(path, encoding="utf-8", errors="replace") as fh:
        for raw in fh:
            line = raw.strip()
            is_comment = in_block
            if in_block:
                if "*/" in line:
                    in_block = False
            elif line.startswith("//"):
                is_comment = True
            elif line.startswith("/*"):
                is_comment = True
                if "*/" not in line:
                    in_block = True
            if not line:
                continue
            if is_comment:
                comment += 1
                run += 1
                longest = max(longest, run)
                if run == RUN_MAX:
                    flagged += 1
            else:
                code += 1
                run = 0
    return code, comment, longest, flagged


def main():
    strict = "--strict" in sys.argv
    sec_hits, review, bloat = [], [], []
    for path in walk():
        code, comment, longest, runs = analyze(path)
        density = comment / code if code else 0
        if (density > DENSITY_MAX and comment > 8) or longest >= RUN_MAX + 3:
            bloat.append((density, longest, comment, code, path))
        with open(path, encoding="utf-8", errors="replace") as fh:
            for i, line in enumerate(fh, 1):
                for rx, msg in SECURITY:
                    if rx.search(line):
                        sec_hits.append((path, i, msg, line.strip()))
                for rx, msg in REVIEW:
                    if rx.search(line):
                        review.append((path, i, msg, line.strip()))

    if sec_hits:
        print("SECURITY (hard-fail):")
        for p, i, msg, src in sec_hits:
            print(f"  {p}:{i}  {msg}\n      {src}")
        print()
    if review:
        print(f"REVIEW ({len(review)} — manual eyeball, non-failing):")
        for p, i, msg, _ in review:
            print(f"  {p}:{i}  {msg}")
        print()

    if bloat:
        bloat.sort(reverse=True)
        print(f"COMMENT BLOAT ({len(bloat)} files — density>{int(DENSITY_MAX*100)}% or long runs):")
        for density, longest, comment, code, path in bloat:
            print(f"  {density*100:4.0f}%  run{longest:<3} {comment:>4}c/{code:<4}L  {path}")
        print()

    fail = bool(sec_hits) or (strict and bool(bloat))
    print(f"{'FAIL' if fail else 'OK'} — {len(sec_hits)} security, {len(bloat)} bloat files")
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main())
