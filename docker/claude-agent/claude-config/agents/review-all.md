---
description: "Run all review agents on target code"
argument-hint: "[FILE_OR_DIRECTORY]"


##Docs:
aid:"./agentic_instructions.md"

##AGENTS:
explorer:"explorer subagent"
supervisor:"supervisor subagent"
unix-coder:"unix-coder subagent"
party:"""

"/supervisor.md"
"/code-review.md"
"/code-simplify.md"
"/datadog-observability-sme.md"
"/dependency-manager.md"
"/merge-manager.md"
"/review-all.md"
"/security-review.md"
"/tech-lead.md"
"/track-manager.md"
"/unix-coder.md"
"/ux-review.md"


##Commands:
ptw:"/plan_with_team"
itw:"/implement_with_team"
di: "/dir_instructions"

\"""

---

# Full Code Review

Run a comprehensive review using all available agents. Launch the following reviews in parallel:

1. **Code Review** (`/code-review $ARGUMENTS`) - Architecture, DRY, extensibility
2. **Security Review** (`/security-review $ARGUMENTS`) - Vulnerabilities, OWASP
3. **Tech Lead Review** (`/tech-lead $ARGUMENTS`) - Architecture, technical debt
4. **UX Review** (`/ux-review $ARGUMENTS`) - Accessibility, interaction design
5. **Code Simplification** (`/code-simplify $ARGUMENTS`) - Complexity reduction

Please run all five review agents on the target and compile a unified report with findings from each perspective.

Target: $ARGUMENTS
