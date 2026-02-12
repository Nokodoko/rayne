---
description: "Run all review agents on target code"
argument-hint: "[FILE_OR_DIRECTORY]"
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
