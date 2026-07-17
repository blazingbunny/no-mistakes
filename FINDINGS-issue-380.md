# Findings: issue #380 root cause, confirmed and reproduced

Investigation date: 2026-07-18
Reproduced on: `no-mistakes v1.37.0` (78e4dcb), `gh 2.45.0` (Ubuntu apt package)

## Symptom

The `ci` pipeline step logs `warning: could not check CI: gh pr checks: exit status 1`
on every poll, forever (up to `ci_timeout`, default 168h), even when the PR's actual
GitHub Actions checks are green.

## Root cause (confirmed, source-located)

`internal/scm/github/github.go`, `Host.GetChecks()`:

```go
func (h *Host) GetChecks(ctx context.Context, pr *scm.PR) ([]scm.Check, error) {
	args := append([]string{"pr", "checks", pr.Number}, h.repoArgs()...)
	args = append(args, "--json", "name,state,bucket,completedAt")
	cmd := h.cmd(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "no checks reported") {
			return nil, nil
		}
		return nil, fmt.Errorf("gh pr checks: %w", err)
	}
	...
```

`--json` on `gh pr checks` was only added upstream in `cli/cli#9079`. Any installed
`gh` older than that release does not support the flag at all:

```
$ gh --version
gh version 2.45.0 (2025-07-18 Ubuntu 2.45.0-1ubuntu0.3)

$ gh pr checks 1 --repo <owner>/<repo> --json name,state,bucket,completedAt
unknown flag: --json
```

This is a **hard CLI usage error**, not a "no checks yet" business-logic response —
but `GetChecks` only special-cases the `"no checks reported"` string. Every other
failure mode, including "unknown flag: --json", falls through to the same generic
`fmt.Errorf("gh pr checks: %w", err)`, which is why the daemon's `ci.log` just shows
an opaque, indistinguishable `exit status 1` on every single poll regardless of cause.

This matches the deeper investigation on #380 by @TheIanLi (PATH-shadow `gh` shim,
2026-07-04), which found the *current* failure mode is this `--json` incompatibility,
not the original cwd/`--repo` theory from #255 (which *was* real and *was* fixed in
v1.26.1, but is a distinct, already-resolved issue).

## Confirmed via reproduction

```
$ gh pr checks 1 --repo blazingbunny/aidens-world-of-games --json name,state,bucket,completedAt
unknown flag: --json
[...]

# After upgrading gh 2.45.0 -> 2.96.0 (GitHub's own apt repo, not Ubuntu's):
$ gh pr checks 1 --repo blazingbunny/aidens-world-of-games --json name,state,bucket,completedAt
[{"bucket":"pass","completedAt":"...","name":"typecheck (...)","state":"SUCCESS"}, ...]
```

Same exact daemon-constructed command, same repo, same PR — only the `gh` binary
version changed.

## Practical fix applied locally

Upgraded `gh` via GitHub's official apt repo:

```bash
sudo mkdir -p -m 755 /etc/apt/keyrings
out=$(mktemp) && wget -nv -O "$out" https://cli.github.com/packages/githubcli-archive-keyring.gpg
sudo install -m 644 "$out" /etc/apt/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list
sudo apt update && sudo apt install gh -y
```

This resolves the symptom completely — Ubuntu's `apt` package for `gh` lags far
behind upstream (2.45.0 vs latest 2.96.0 at time of writing), so anyone on a
Debian/Ubuntu box installing `gh` via the distro package manager is likely to hit
this regardless of `no-mistakes` version.

## Suggested upstream fix (not yet submitted as a PR)

`GetChecks` should distinguish a `gh` CLI usage error from a genuine "no checks"
or "still pending" response, e.g.:

- Check `gh --version` (or attempt a `--json` capability probe) once at daemon
  startup and log a clear, actionable error ("your installed gh is too old for
  --json support on pr checks, please upgrade") instead of silently retrying
  the same doomed command forever.
- Or: detect `"unknown flag: --json"` / `"unknown flag"` specifically in
  `GetChecks`'s error branch and surface it as a distinct, non-retryable error
  rather than folding it into the generic retry-forever `ci` step loop.

Either would have turned a 33+ minute silent hang into an immediate, actionable
error message.

## Related upstream issues

- [#255](https://github.com/kunchenguid/no-mistakes/issues/255) (closed, v1.26.1) —
  original cwd/`--repo` bug, fixed for real.
- [#380](https://github.com/kunchenguid/no-mistakes/issues/380) (open, reported
  v1.31.2, still reproduces on v1.37.0) — this write-up's subject.
- [#475](https://github.com/kunchenguid/no-mistakes/issues/475) (open) — separate:
  CI-watch polls indefinitely when a repo has *no* CI configured at all (should
  short-circuit to "checks-passed"-equivalent instead of waiting out the full
  timeout).
