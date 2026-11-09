# Proposal

## Why

Users with many SSO profiles (25 and more is common in multi-account organizations) get a picker that fills the whole terminal and must be walked with the arrow keys. The underlying `huh` select already supports filtering behind the `/` key, but nothing tells the user about it, so in practice the list cannot be searched.

## What Changes

- The picker opens with the filter input focused: typing immediately narrows the list. Matching is a case-insensitive substring match against the whole row, so it finds profiles by name, account ID or role name.
- The picker shows a hint under the input: `Type to filter by name, account ID or role`.
- The picker has a fixed height of 15 lines (input and hint included). Longer lists scroll instead of filling the terminal.
- `Esc` clears the filter. Arrow keys move, `Enter` confirms, `Ctrl+C` cancels, as before.

User-visible impact per command:

- `console`, `export`, `import`: the picker, shown when no profile comes from `--profile` or `AWS_PROFILE`, filters as the user types and is at most 15 lines tall. It is still drawn on stderr, so `export` stdout stays `eval`-safe.
- `process`: no change; it never shows the picker.

No flags, defaults, stdout formats or exit codes change. Nothing in this change is **BREAKING**.

## Out of scope

- Fuzzy matching (for example `hvgp` matching `hipaa-vg-prod`). `huh` v1.0.0 has no pluggable filter function, and substring matching covers the reported need.
- A positional filter argument or `--filter` flag that preselects or prefilters profiles.
- Ordering by recent use or any persisted picker state.
- Integration with external tools such as `fzf`.

## Related issues

None found (searched `gh issue list --state all` for "picker filter search" and "profile selection").

## Impact

- Code: `src/internal/cli/selector.go` (picker construction), `src/internal/cli/selector_test.go`.
- Docs: `README.md` (interactive selection section), `CHANGELOG.md` (`Unreleased`).
- Specs: `profile-resolution` gains a requirement for filtering and height.
- Dependencies: none added.
