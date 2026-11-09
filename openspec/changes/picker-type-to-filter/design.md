# Design

## Context

`HuhSelector.Pick` in `src/internal/cli/selector.go` builds a `huh.Select[string]` whose option labels are `name  accountID  roleName` and runs it in a form drawn on stderr. `huh` v1.0.0 ships a filter for `Select`: `/` focuses a text input and options are kept when their label contains the input, compared case-insensitively. `Filtering(true)` starts the field with that input focused, and `Height(n)` caps the field at `n` lines and makes the option list scroll.

## Decisions

1. **Use the built-in `huh` filter.** Call `Filtering(true)` so typing filters right away. Because the label already contains the account ID and role name, one substring match covers all three columns. No new dependency is needed.
2. **Fixed height of 15 lines.** `Height(15)` keeps the picker usable on small terminals and stable while the list shrinks during typing. The input and the hint take two of the 15 lines, so 13 profiles are visible at once.
3. **Hint instead of title while filtering.** When filtering is active, `huh` draws the filter input in place of the title. A `Description` line (`Type to filter by name, account ID or role`) keeps the picker self-explanatory.
4. **Extract the form builder.** Move the form construction into `newProfileForm(profiles, *choice) *huh.Form`. `Pick` only adds `WithOutput(os.Stderr)` and runs it. Tests drive the returned form with `tea.KeyMsg` updates and inspect `View()` without a terminal.

Rejected: a custom `bubbles/list` model with fuzzy matching (more code to own and test for little gain), and `fzf` integration (external dependency, different behavior per machine).

## Interaction with AWS CLI files

None. The profile list still comes from `profiles.ListSSOProfiles` reading the AWS config file (`AWS_CONFIG_FILE` or `~/.aws/config`). Credentials and the AWS CLI cache are untouched.

## Platform differences

None between macOS and Linux. The picker still requires stdin and stderr to be terminals, checked by the existing `isInteractive` seam. Windows stays untested.

## Testability

The picker UI is not run in tests. `newProfileForm` returns the `huh.Form` model, which tests feed with a window size message and key messages, then assert on `View()`: which profiles are listed after typing a name fragment, an account ID fragment and a role fragment, and that the hint is shown. Profile data uses placeholders such as `123456789012`. No real `~/.aws` files are read.
