# Local native source SAST gate

This gate runs established analyzers without network bootstrap, source execution,
registry changes, global tool installation or source upload. It is required for maintained Python/Bash source changes before dev promotion. Hosted CI does not acquire a new gate automatically.

## Prepare isolated tools

The captured wheel lock targets Linux aarch64 and Python 3.12. Other platforms need a
separately reviewed complete wheel lock; do not replace hashes or use an unpinned
installer to make this lock pass. Prepare under the checkout's `.security/analyzers`.

```sh
mkdir -p .security/analyzers/bandit-1.9.4/wheels
python3 -m pip download --no-cache-dir --only-binary=:all: --require-hashes \
  --dest .security/analyzers/bandit-1.9.4/wheels \
  -r scripts/security/requirements-bandit.lock
python3 -m venv .security/analyzers/bandit-1.9.4/venv
.security/analyzers/bandit-1.9.4/venv/bin/python -m pip install \
  --no-index --no-cache-dir --require-hashes \
  --find-links .security/analyzers/bandit-1.9.4/wheels \
  -r scripts/security/requirements-bandit.lock
```

The lock records Bandit 1.9.4 and every resolved dependency wheel hash. Record actual
wheel bytes, installed distribution identities and the installer exit before scanning.
Bandit's fixed release comes from [PyCQA](https://github.com/PyCQA/bandit/releases/tag/1.9.4);
its [official CLI](https://bandit.readthedocs.io/en/latest/man/bandit.html) documents AST
plugins and report options.

Download the [official ShellCheck v0.11.0 Linux aarch64 archive](https://github.com/koalaman/shellcheck/releases/tag/v0.11.0)
with HTTPS to `.security/analyzers/shellcheck-v0.11.0.linux.aarch64.tar.xz`.
Verify SHA-256 `12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588`.
Inspect archive members to reject absolute paths, traversal and links, then copy only
its regular `shellcheck-v0.11.0/shellcheck` member into `.security/analyzers/` and make
that owned executable runnable. Do not pipe downloaded content into a shell or replace
system binaries.

## Run and interpret

```sh
python3 scripts/security/source_sast.py \
  --out "$PWD/.security/source-sast-unique-candidate"
python3 -m unittest discover -s scripts/security -p 'test_source_sast.py' -v
```

The output path must be new and inside the checkout's `.security`; existing evidence
is never overwritten. Tool paths can be explicitly supplied with `--bandit-python`
and `--shellcheck`; exact native tool versions are checked. No command downloads tools.

All tracked `.py` and `.sh` files are scanned. Literal quoted Python heredocs and the
known `FAKE` test dispatcher string are copied without evaluation and receive original
source/hash/line mappings. Dynamic, ambiguous or unsupported heredocs/generated Python
fail closed. Bandit analyzes extracted Python; extraction itself is not a custom SAST
engine. ShellCheck analyzes shell only, not embedded Python.

Exit 2 means unavailable/invalid tools, reports, source inventory or extraction. Exit 1
means a Bandit HIGH or ShellCheck error finding. Other findings are retained for
non-author applicability review; policy exit 0 with warnings is explicitly `raw_clean=false`
and is not analyzer clean or completed independent review. Native analyzer exit codes,
full reports, tool/environment hashes and before/after source hashes remain in receipts.
No `--exit-zero`, global suppressions, baseline deletion or ignored findings are used.
