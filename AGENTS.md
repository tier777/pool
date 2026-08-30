## Git and GitHub

Use GitHub Flow: `main` is the integration branch and must remain ready to
release. For each change, create a short-lived branch from the current `main`,
make the change, run relevant checks, open a pull request, and merge it into
`main`. Delete the branch after merge.

Before researching, planning, or changing files, synchronize the local branch
with its remote. If the working tree is clean, run `git pull --ff-only`.

Keep commits focused. Do not include unrelated local changes, generated
credentials, secrets, or machine-specific files. Review the final diff before
committing and pushing. Never force-push or rewrite shared history.

GitHub branch protection is optional. Do not block work, request a GitHub plan
upgrade, or change repository settings because branch protection is unavailable.

## GitHub Project

Use [Pool project](https://github.com/users/tier777/projects/5)
as the single development board. Every planned code change starts as a GitHub
issue and is added to the project.

- Before implementation, set the item to `In progress`; create the branch from
  current `main` as `codex/<issue-number>-<short-name>`.
- Keep the issue focused: state the outcome and acceptance criteria. Set its
  `Priority` and `Size` when known; do not invent estimates.
- Open a pull request that links the issue (for example, `Closes #123`). After
  its checks and merge, move the project item to `Done` and close the issue.
- Use `Todo` for approved work not yet started. Do not create project items for
  incidental investigation or one-off maintenance with no implementation.

## Documentation

Keep relevant documentation accurate when a change makes it outdated. Do not
create new documentation by default. Never add secrets or private operational
data to documentation.
