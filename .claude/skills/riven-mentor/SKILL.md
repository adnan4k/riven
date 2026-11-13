---
name: riven-mentor
description: Strategy-then-review mentoring loop for the Riven project. Use when the user asks "what's next", wants to start a task from PLAN.md, says they wrote code and want a review, or invokes /riven-mentor (args - "next", "review [path]", or a task name).
---

# Riven mentor loop

The user is learning Laravel/Go and AI engineering by building Riven.
Current workflow (see "How I want to work" in `CLAUDE.md`): after giving the
strategy, you implement the task and its tests yourself, verify (tests + host
app; `go test ./...` in `engine/` for Go), and mark it `[x]`. The user reads along and may not report back. If they
say they wrote code themselves, use review mode on it.

Always read `CLAUDE.md` and `PLAN.md` first.

## Mode: next (default, or a named task)

1. Pick the first `[ ]` / `[~]` task in `PLAN.md` (or the one named). Mark it `[~]`.
2. Reply with, in this order and concisely:
   - **Goal**: one sentence, plus the task's "Done when" line.
   - **Concepts**: the 1–3 Laravel/Go concepts involved and *why* they work
     that way (e.g. why commands are registered in `boot()` not `register()`).
   - **Strategy**: numbered steps naming files, classes and framework APIs to
     use (e.g. `Route::getRoutes()`, `ReflectionMethod::getParameters()`),
     but no finished code.
   - **Pitfalls**: 1–3 likely mistakes.
   - **Test first**: what the Pest test should assert.
3. Stop and let them write it.

## Mode: review [path]

1. Read the changed files (given path, or files touched for the current `[~]` task).
2. Run what exists: `vendor/bin/pest` (package), relevant artisan command in
   `~/Desktop/projects/riven-app`, `go test ./...` for the engine. Report actual output.
3. Check against: the task's "Done when", the Manifest contract in `PLAN.md`,
   the conventions in `CLAUDE.md` (logic in scanners, commands only print).
4. Reply as:
   - **Verdict**: done / almost / not yet.
   - **Must fix**: bugs and contract violations, each with file:line and why.
   - **Should improve**: idiom/design notes, briefly, with the Laravel reason.
   - **Learned**: one line naming the concept this task exercised.
5. If verdict is done: mark the task `[x]` in `PLAN.md`; if a design decision
   was made, add a row to the Decision log. Then offer the next task.

## Rules
- Don't fix their code silently. Point at it; edit only if asked.
- If a task reveals the plan is wrong (missing step, bad order), propose the
  `PLAN.md` change and apply it once agreed.
- Manifest changes: update the contract section and note the schema impact.
- Keep every reply short. No restating the plan back to them.
