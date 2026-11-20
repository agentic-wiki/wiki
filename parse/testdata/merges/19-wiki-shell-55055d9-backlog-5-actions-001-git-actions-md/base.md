
The header's three actions. **Refresh** rebuilds the index. **Pull** is `git pull --rebase`. **Sync** is commit and push.

These are the first things wikiview does that reach outside the machine, and the first that are hard to undo. They are shaped accordingly.

## Preview, then confirm

Every action shows what it will do before it does it: files changed, commits ahead and behind, the commit message. Then it acts on confirmation.

Not ceremony. A push is irreversible from this UI's point of view, and a rebase that stops halfway leaves a conflicted working tree that a web page has no business trying to resolve.

## When a pull fails

The rule, decided rather than discovered later:

1. **Undo the attempt** — `git rebase --abort`, restoring the tree exactly as it was before. The UI never leaves the user in a conflicted state it cannot get them out of.
2. **Offer to push the local work as a new branch**, with a proposed name (`wikiview/2026-08-10-1430`) the user can confirm or override.

That turns a merge conflict from "your knowledge base is now broken and you are in a web page" into "your work is safe on a branch, resolve it where you have a real tool". It is the difference between an action that can strand someone and one that cannot.

## Constraints worth stating

- **Git is optional.** A bundle is a folder; it need not be a repository, and `git` need not be installed. The actions are absent, not broken, when it is not.
- **The bundle root is not necessarily the repository root.** A bundle can be a subdirectory of a larger repo, so committing must scope to the bundle rather than sweeping the whole worktree.
- **Nothing is automatic.** No pull on load, no push on a timer. The bundle belongs to the user and to whatever agent is also editing it; surprising them with a network operation is worse than making them click.
- **A rebuild follows a pull**, since the files changed underneath the index. The watcher will notice anyway; doing it deliberately avoids a window where the UI shows the old content.

## Open

Whether `sync` should refuse when the working tree contains changes the user did not make through wikiview. Leaning no — it is their repo and their folder, and an agent editing alongside is the expected case, not an anomaly — but the preview must show everything that will be committed, not only what wikiview touched.

**Acceptance:** each action previews before acting; a failed pull restores the previous state and offers a named branch; the actions are hidden when the bundle is not a git repository; committing is scoped to the bundle; nothing runs without being asked.

## Done

`internal/git` shells out to the git binary. Not for want of a library: these commands reach a remote, and reaching a remote means the user's credential helper, ssh agent, ssh config, proxy settings and whatever their gitconfig says about signing and hooks. A library reimplements that stack badly or asks a wiki viewer to hold credentials. Shelling out also makes "git is optional" true for free — no binary, no actions.

**`GET /api/git` costs no network.** It reads the branch, the upstream, ahead/behind as of the last fetch, and everything a commit would carry. `POST /api/git/fetch` is the one read that reaches out, and it happens when somebody opens the pull preview: asking to see what a pull would do is asking.

**A failed pull answers with the way out.** `409`, git's own words, the repository as it now stands, and the proposed branch name — all in one body, because a client just told an action failed needs to know where that left things, and asking again would ask about a different moment.

**Refresh does not preview.** It re-reads the disk, reaches nothing and undoes nothing; a preview of "I will look at the files again" is ceremony, and the rule it would be obeying exists for the two actions that can strand somebody. It also survives when the bundle is not a repository, because it has nothing to do with git.

**A sync does not rebuild.** A commit does not change a working file, so there is nothing for the index to re-read. Only a pull moves content under the reader, and that one rebuilds before answering rather than leaving a window where the screen shows the old content.

**Signing is never dropped to make a commit succeed.** Somebody who set `commit.gpgsign` decided their commits are signed, and quietly producing unsigned ones would put a hole in that record they did not ask for and would not notice. It would also make this behave differently from the same command in their terminal, which is the one thing shelling out is for, and it often only moves the failure to a repository that requires signatures. The commit fails, and the message names signing rather than leaving "Couldn't get agent socket" to be interpreted.

**The commit message is proposed from what changed**: the path when one file moved, a count and their shared folder when several did. Not an empty box, which asks somebody to name work they watched an agent do; and not a date, which git already records — putting the timestamp in the subject line duplicates metadata git owns while still saying nothing about the change. It is a starting point in an editable field.

**Every button names what it is doing.** "Pulling…" and "Pushing…" rather than one "Working…" covering both, and the read that opens a pull preview is a third thing again: it holds its own state, so the confirm button is disabled but never claims to be pulling while it fetches.

**A sync with nothing to commit is a push, and says so.** For a bundle whose commits were made in a terminal there is nothing to say about a commit that is not happening, so no message is asked for, the field is not rendered, and the button reads "Push". A dialog that finished its work closes itself after a moment, since it has nothing left to say. The exception is a rescue: the branch name is the whole point of it, and dismissing would take away the only place it is written down.

**Five bugs the real repositories caught**, none of which a mock would have:

- `run` trimmed all whitespace, and `git status --porcelain` puts significant spaces in its first two columns: ` M index.md` became `M index.md` and every path lost its first character.
- Git collapses an untracked folder to one `?? notes/` line. A preview that says "a directory" is not a preview, so it asks for `--untracked-files=all`.
- `git push --set-upstream=false` is not valid; the flag takes no value. Dropped entirely, since a rescue branch is a place to put work rather than one to move onto.
- **`add -- .` scoped the staging, but the commit swept the whole index.** `git commit` with no pathspec commits everything staged, so a bundle sitting inside a larger repository put whatever somebody had staged in their terminal into the wiki's commit. Every step now carries the bundle's pathspec, and the preview says how many staged files it is stepping around: not committing them is right, and letting somebody assume otherwise after pressing "commit and push" is not. A count rather than the paths, since the place to look at somebody else's work is `git status`.
- A sync refused for want of a message had already staged the bundle, leaving work staged by a button nobody got to press. It asks first and stages after.

**Left as the task decided it.** A sync does not refuse when the worktree holds changes nobody made through wikiview: it is the user's repo, and an agent editing alongside is the expected case. The preview showing everything that would be committed is the honest half of that bargain.
