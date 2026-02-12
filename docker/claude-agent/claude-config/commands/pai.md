# Pai

---

ARGUMENTS:

- instructions for the plan_with_team command, which should include the following:
  - A clear and concise description of the task or project that needs to be completed.
  - Any specific goals or objectives that need to be achieved.
  - Any constraints or limitations that should be considered during the planning process.
  - Any relevant background information or context that may help the team understand the task better.
  - A list of any resources or tools that may be needed to complete the task successfully.md

##Docs:
aid:"./agentic_instructions.md"

##AGENTS:
explorer:"explorer subagent"
supervisor:"/supervisor.md subagent"
unix-coder:"/unix-coder.md"
reviewer:"/review-all.md"

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

"""

##Commands:
ptw:"/plan_with_team"
itw:"/implement_with_team"
di: "/dir_instructions"
pai: "/plan_and_implement_with_team"

---

argument-hints: [
"Provide clear and concise instructions for the plan_with_team command, including a description of the task, goals, constraints, background information, and necessary resources."
]

## Step 1: Create Plan with Supervisor

1. RUN: /plan_with_team.md $ARGUMENTS

## Step 2: Implement Plan with Supervisor

2a. Take the plan output from `/plan_with_team.md` ({{ aid }}) and use it to create a new command file named `dir_instructions.md`. This file should contain the entire plan output, including team member definitions and step by step tasks. This command file will be passed to the supervisor sub-agent in the next step.

3. RUN: /implement_with_team.md
