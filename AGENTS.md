# Agents

This document helps AI agents work effectively in this codebase. It explains the philosophy, patterns, and pitfalls behind the code, so you can make good decisions on any task, not just scenarios explicitly covered.

## Philosophy

This is a minimal, idiomatic Tesla MCP project. Simplicity is a feature.

Before adding code, articulate why it's necessary in one sentence. If you cannot justify the addition, it probably isn't needed.

Before adding a dependency, don't.

## Workflow

Every task follows phases. Do not skip phases; if you realize you missed something, return to the appropriate phase.

**Making changes:**

1. **Research** - Understand the problem and codebase before acting
2. **Plan** - Articulate your approach before implementing (when asked, or for complex changes)
3. **Implement** - Make changes in small, verifiable steps
4. **Verify** - Confirm correctness using external tools

For trivial changes (typo fixes, comment updates), an abbreviated workflow is acceptable.

## Research

Research in sequential passes. Each pass has one focus. Don't skip ahead to code until you've completed the earlier passes.

**Pass 1: Read the issue.** If you were given a link, read it now. Do not explore code, do not pass go. Fetch the linked issue or document first. Summarize what it asks for. If you cannot restate the problem, you are not ready to proceed.

**Pass 2: Read linked references.** Follow every link in the issue: related issues, RFCs, external docs. Document what you learn.

**Pass 3: Trace the code.** Start from public API inward. Read tests for intent.

**Pass 4: Search exhaustively.** If the change affects a pattern used in multiple places, grep for all instances. Missing one creates inconsistent behavior.

**Pass 5: Document unknowns.** List what you still don't know. Unknown unknowns become known unknowns when you ask "what am I still unsure about?"

**After all passes:** Can you restate the problem in your own words? If not, return to Pass 1. Gaps in earlier passes will cause problems later.

## Plan

When asked to plan, write to `PLAN.md`. Write for someone else, not yourself; don't skip context you already know.

**Pass 1: Document research.** Summarize what you learned in Research. Follow every reference; document findings so the implementer can verify. If the research section is empty, you haven't researched enough.

**Pass 2: Consider approaches.** For non-trivial problems, enumerate at least two approaches. For each, note: what would change, what could go wrong, what's the tradeoff.

**Pass 3: Detail the chosen approach.** Explain what and why, not step-by-step how. Point to specific files, functions, line numbers. Make claims verifiable. Leave room for the implementer to find a better solution.

**Pass 4: List open questions.** What's still unclear? What assumptions are you making? What would change your approach?

**After all passes:** Review from the implementer's perspective. Could they start work with only this document? If not, add what's missing.

## Implement

Implement in sequential passes. Don't write code until you've completed the verification passes.

**Pass 1: Verify understanding.** Did you do your research? Can you state your approach in one sentence? If requirements are ambiguous, stop and ask. A wrong assumption wastes more time than a quick question.

**Pass 2: Check scope.** Does this need to exist? Check if it already exists in the API. Is this the library's job or the user's job? The library handles protocol correctness; application concerns (reconnection, auth, routing) belong in user code.

**Pass 3: Check invariants.** Walk through Key Invariants before writing code:

- Context propagation: Are you using `context.Context` propagation through the entire call chain?
- Protocol: Does implementation remain compliant with the MCP/JSON-RPC contract?
- Encapsulation: Do Tesla API details stay behind domain boundary and do not leak into  MCP handling.

**Pass 4: Implement.** Make the change. Every change needs a reason; if you can't articulate why it improves things, don't make it. Preserve existing comments unless you can prove they're wrong.

**Pass 5: Verify examples.** Trace through usage examples as if writing real code. If an example wouldn't compile, the design is wrong. Check edge cases: what happens on error? On cancellation?

**After all passes:** If feedback identifies a problem, fix that specific problem. Don't pivot to a new approach without articulating what failed and why the new approach avoids it.

## Verify

Verify using external signals. Self-assessment is unreliable; these tools are the ground truth.

**Pass 1: Run tests.** `go test ./...` must pass. If tests fail, return to Research to understand why, not to Implement to "fix" it.

**Pass 2: Run vet.** `go vet ./...` must report no issues.

**After all passes:** If any pass fails, fix the issue and restart from Pass 1. Don't consider a change complete until all passes succeed.

## When Uncertain

If confidence is low or requirements are ambiguous:

1. State what you're uncertain about
2. Identify what information would resolve the uncertainty
3. Gather that information or ask for clarification

Do not proceed with low-confidence assumptions.

For complex changes, approach from multiple angles. If two approaches give different answers, that discrepancy demands resolution before proceeding.

## Code Style

Follow Go conventions: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), and [Go Proverbs](https://go-proverbs.github.io/). Be concise, declarative, and factual.

Never use emdash. Use commas, semicolons, or separate sentences.

**Inline comments** are terse. Prefer end-of-line when short enough.

**Explain why, not what.** The code shows what it does; comments should explain reasoning, non-obvious decisions, or edge cases.

**Wrap comments** at ~80 characters, continuing naturally at word boundaries.

**Naming matters.** Before proposing a name, stop and review existing names in the file. Ask: what would someone assume from this name? Does it fit with how similar things are named? A good name is accurate on its own and consistent in context.

**Comment content:**

- Add information beyond what the code shows (not tautologies)
- State directly: "Returns X" (not "Note that this returns X")
- Drop filler: "basically", "actually", "really" add nothing


## Commits and PRs

Use description only commits: `description`.

**Before writing the commit message**, articulate what was wrong with the previous behavior and why this change fixes it. The commit message captures this reasoning.

**Commit messages explain why.** State what was wrong, why it mattered, and why this solution was chosen. One to three sentences. Keep the tone neutral.

**PR descriptions are for humans.** Explain what reviewers can't see: the why, alternatives considered, and tradeoffs made. Show evidence (logs, benchmarks, screenshots). Be explicit about what the PR doesn't do.




