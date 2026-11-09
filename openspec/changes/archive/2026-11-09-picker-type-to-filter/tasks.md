# Tasks

Each group is one logical change. It ends with its own commit after `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate picker-type-to-filter --strict` pass. Tests never touch the real `~/.aws` and never run the real AWS CLI.

## 1. Proposal

- [x] 1.1 Write proposal, design, the `profile-resolution` spec delta and tasks, and run `openspec validate picker-type-to-filter --strict`.
- [x] 1.2 Commit `docs: propose type-to-filter profile picker` (no related issues).

## 2. Type-to-filter picker

- [x] 2.1 Extract `newProfileForm(profiles, *choice) *huh.Form` in `src/internal/cli/selector.go` and enable `Filtering(true)`, `Height(15)` and the hint `Type to filter by name, account ID or role`. `Pick` keeps drawing on stderr and mapping `huh.ErrUserAborted` to the cancellation error.
- [x] 2.2 Add unit tests in `src/internal/cli/selector_filter_test.go` that drive `newProfileForm` with key messages and assert on `View()`: the hint is shown, typing a name fragment, an account ID fragment and a lower-case role fragment keeps only matching profiles, legacy and sso-session profiles both match, a non-matching filter lists none, and a long list shows at most 13 rows. Use placeholder account IDs.
- [x] 2.3 Update the interactive selection section of `README.md` and add an `Added` entry to the `Unreleased` section of `CHANGELOG.md`.
- [x] 2.4 Commit `feat: filter the profile picker as you type` (no related issues).

## 3. Verification and PR

- [x] 3.1 Run `make tests`, `go vet ./...`, `gofmt -l ./src` (no output) and `openspec validate picker-type-to-filter --strict`, then `openspec validate --all --strict`.
- [x] 3.2 Open the PR. The description links the change id `picker-type-to-filter` and states that no issues are related.
