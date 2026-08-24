# PR Stacks (gh-stack)

**Always use stacked PRs.** Every feature branch is one layer in a stack; each PR's base is the branch below it, so reviewers see only that layer's diff. The bottom of the stack targets `main`. A new branch is created with `gh stack add` (never a bare `git checkout -b` off the stack).

Install + alias:
```bash
gh extension install github/gh-stack   # one-time
gh stack alias                          # optional: shortens to `gs`
```

Daily commands:
```bash
gh stack init [--base main]          # start a stack from trunk (adopt existing branches with `gh stack init a b c`)
gh stack add <branch>                # new layer on top of the current branch (must be on the topmost layer)
gh stack add -Am "commit msg" <branch>  # stage, commit, branch in one step (branch optional, auto-named)
gh stack submit --auto --open        # push all branches, create PRs, link them as a Stack on GitHub
gh stack push                        # push branches (force-with-lease per branch)
gh stack rebase                      # cascade-rebase the stack on trunk (resolve → `git add` → `gh stack rebase --continue`)
gh stack sync --prune                # fetch, rebase, push, sync PR state; prune merged branches
gh stack view                        # show stack layers + PR links
gh stack checkout <n>                # check out by stack/PR number or branch
gh stack up / down / top / bottom    # navigate layers
gh stack merge [n]                   # merge PRs up to and including layer n (all-or-nothing)
gh stack unstack                     # remove stack tracking (keeps branches/PRs)
```

Rules:
- Stack metadata lives in `.git/gh-stack` (JSON, never committed). Rebase state in `.git/gh-stack-rebase-state`.
- `gh stack add` must run while on the stack's topmost branch.
- `gh stack submit` in non-interactive/CI mode needs `--auto` (drafts) or `--open` (ready for review).
- After a reviewer asks for changes: `gh stack bottom` → fix → commit → `gh stack rebase` → `gh stack push`.
- The bottom PR carries the phase scope per the PR-title rule; each layer above gets the next phase number.
- The existing phase stack (PRs 1–19, branches `01-scaffold-db` … `plugins-marketplace`) is managed this way — keep adding layers with `gh stack add`, not new root branches.
