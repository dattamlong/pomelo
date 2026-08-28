---
name: pom-review
description: Author a Pomelo multi-repo review for the current workspace — a narrative that links each claim to real code across every repo in the workspace. Use when asked to "review this workspace/branch" or "write a review" inside a Pomelo workspace.
---

# pom-review

Write a review artifact that Pomelo's Review tab renders: concise prose about intent,
architecture, data flow, and risk, with every claim linked to real code via repo-
qualified anchors. A Pomelo workspace spans several repos on one branch, so anchors
must name the repo.

## Where to write

The artifact is one JSON file at the workspace/project root:

```
<project-root>/.pom/reviews/<branch>.json
```

Find `<project-root>`: from any repo in the workspace, run `git rev-parse --show-toplevel`
to get the repo dir, then walk up until you find the directory that contains `.pom/`
(that is the project root; the repos sit under it or under `workspace--<branch>/`).
`<branch>` is the workspace branch — `git rev-parse --abbrev-ref HEAD` in any repo.
Create `.pom/reviews/` if missing.

## What to inspect

For each repo in the workspace, read what the branch changed against its base:

```
git -C <repo> diff -M origin/<default-branch>...HEAD      # files + hunks
git -C <repo> log origin/<default-branch>..HEAD --oneline # commits
```

Resolve `<default-branch>` per repo (it may be `main` in one repo and `master` in
another) via `git -C <repo> symbolic-ref --short refs/remotes/origin/HEAD`.

## Schema

```json
{
  "exists": true,
  "id": "<short-slug>",
  "title": "<one line>",
  "doc": "<markdown>",
  "anchors": [
    { "id": "a1", "repo": "<repo dir name>", "path": "<path within that repo>",
      "start": <first line>, "end": <last line>, "side": "head" }
  ]
}
```

- `doc` is GitHub-flavored markdown. Link a phrase to code with an anchor URL:
  `[the phrase](pom://code?repo=<repo>&path=<path>&start=<n>&end=<m>)`.
  The `repo`/`path`/`start`/`end` MUST match an entry in `anchors`.
- `repo` is the repo's directory name as it appears in the workspace (e.g. `api`,
  `web`, `worker`) — the same name Pomelo shows. `path` is relative to that repo.
- Prefer a handful of high-signal anchors over many. Keep the prose tight: what the
  change does, why, the data flow across repos, and the risks a reviewer should check.

## Rules

- VALIDATE every anchor before writing: the file exists in that repo and the line
  range is within the file. Read the file to confirm, do not guess line numbers.
- Repo-qualified always — never emit an anchor without a `repo`.
- Plain ASCII, no emoji.
- Overwrite the file if it already exists (a review is regenerated per branch).

## After writing

Tell the user the review is ready and to open the Review tab (Cmd-5) on this
workspace in Pomelo.
