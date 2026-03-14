# Changelog

## v0.2.2

### Bug Fixes

- **Filter input no longer drops keystrokes during background key loading.**
  Previously, when the application was still loading keys from the database
  (pagination), typing a filter could lose characters because `SetItems`
  was called on the list while the filter input was active.
  Incoming keys are now buffered and flushed only after the filter
  input is closed.

- **All records are restored after clearing a filter.**
  Clearing the filter (Esc) previously left the list stuck at the
  last loaded page — remaining keys were never fetched.
  Normal pagination now resumes immediately when the filter is removed.

## v0.2.1

Initial public release.
