# Implement With Team

---

##Docs:
aid:"./agentic_instructions.md"

##AGENTS:
explorer:"explorer subagent"
supervisor:"/supervisor.md subagent"
unix-coder:"/unix-coder.md"
reviewer:""/review-all.md"

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

---

## Step 1: Spawn the supervisor

Immediately Launch a single **general-purpose** sub-agent as the {{ supervisor }}. Pass it the ENTIRE contents of this command file plus the raw `$ARGUMENTS` string. The supervisor self-bootstraps by reading the command prompt to understand its mission. The base orchestrator waits for it to return.

**The base orchestrator's context is now completely protected.** It contains only: the raw `$ARGUMENTS` string, any hook interactions, and the final summary. No directory listings, file contents, code, phase definitions, or sub-agent outputs ever enter its context.

## Instructions

- Consider edge cases, error handling, and scalability concerns
- Understand your role as the team lead. Refer to the `Team Orchestration` section for more details.

### Team Orchestration (collapsed)

## Workflow

IMPORTANT: **PLANNING ONLY** — Do not execute, build, or deploy. Output is a plan document.

1. Analyze Requirements — Parse the PLAN to understand the core problem and desired outcome
2. Understand Codebase — With each {{ explorer }} on tracks directly understand existing patterns, architecture, and relevant files by referencing the local {{ aid }}.
3. Design Architecture — Create a technical approach including architecture decisions and implementation strategy
4. Define Team Members — Use {{ ptw }} (if provided) to guide team composition. Identify from {{ party }} or use `general-purpose`
5. Define Step by Step Tasks — Use {{ ptw }} (if provided) to guide task granularity and parallel/sequential structure. Write out tasks with IDs
6. Generate Filename — Create a descriptive kebab-case filename based on the plan's main topic
7. Save & Report — Follow the `Report` section to write the plan to `PLAN_OUTPUT_DIRECTORY/<filename>.md` and provide a summary of key components

#### Task Management Tools

note these are examples! The titles of each task and the output should be the same. The values of each key will differ based on the specific task and context.

`typescript
    TaskCreate({
      subject: "Implement user authentication",
      description: "Create login/logout endpoints with JWT tokens. See specs/auth-plan.md for details.",
      activeForm: "Implementing authentication"  // Shows in UI spinner when in_progress
    })
    // Returns: taskId (e.g., "1")
    `

**TaskUpdate** – Update task status, assignment, or dependencies:
`typescript
    TaskUpdate({
      taskId: "1",
      status: "in_progress",  // pending → in_progress → completed
      owner: "{{ unix-coder }}-auth"   // Assign to specific team member
    })
    `

**TaskList** – View all tasks and their status:
`typescript
    TaskList({})
    // Returns: Array of tasks with id, subject, status, owner, blockedBy
    `

**TaskGet** – Get full details of a specific task:
`typescript
    TaskGet({ taskId: "1" })
    // Returns: Full task including description
    `

## Plan Format

- This is critical. Your job is to act as a high level director of the team.
- Your role is to validate all work is going well and make sure the team is on track.
- You'll orchestrate this by using the Task\* Tools to manage coordination.
- Communication is paramount. You'll use the Task\* Tools to communicate with team members.
- Take note of the session id of each team member. This is how you'll refer to them.

### Team Members

<list the team members you'll use to execute the plan>

- {{ unix-coder }}
- Name: <unique name for this {{ unix-coder }} - this allows you and other team members to identify>
- Role: <the single role and focus of this {{ unix-coder }} will play>
- Agent Type: <the subagent type of this {{ unix-coder }}, you'll specify based on requirements>
- Resume: <default true. This lets the agent continue working with the same context>
  <continue with additional team members as needed in the same format as above>

## Step by Step Tasks

- IMPORTANT: Execute every step in order, top to bottom. Each task maps directly to a TaskCreate call.
- Before you start, run `TaskCreate` to create the initial task list that aligns with the plan.

<list step by step tasks as h3 headers. Start with foundational work, then build up>

### 1. <First Task Name>

- **Task ID**: <unique kebab-case identifier, e.g., "setup-database">
- **Depends On**: <Task ID(s) this depends on, or "none" if no dependencies>
- **Assigned To**: <team member name from Team Members section>
- **Agent Type**: <subagent from TEAM_MEMBERS file or GENERAL_PURPOSE_AGENT>
- **Parallel**: <true if can run alongside other tasks, false if must be sequential>
- <specific action to complete>
- <specific action to complete>

### 2. <Second Task Name>

- **Task ID**: <unique-id>

### Right Panel: hooks-update-with-team.md — Team Members

- You'll orchestrate this by using the Task\* Tools to manage coordination.
- Communication is paramount. You'll use the Task\* Tools to communicate with team members.
- Take note of the session id of each team member. This is how you'll refer to them.

### Team Members

- {{ unix-coder }} (SessionEnd Hook)
- Name: session-end-{{ unix-coder }}
- Role: Implement the SessionEnd hook with logging and cleanup capabilities
- Agent Type: {{ unix-coder }}
- Resume: true

- {{ reviewer }} (SessionEnd Hook)
- Name: session-end-validator
- Role: Validate SessionEnd hook works correctly using `claude -p` and test scripts
- Agent Type: validator
- Resume: true

- {{ unix-coder }} (PermissionRequest Hook)
- Name: permission-request-{{ unix-coder }}
- Role: Implement the PermissionRequest hook with allow/deny decision capabilities
- Agent Type: {{ unix-coder }}
- Resume: true

- {{ reviewer }} (PermissionRequest Hook)
- Name: permission-request-validator
- Role: Validate PermissionRequest hook works correctly
- Agent Type: validator
- Resume: true

- {{ unix-coder }} (PostToolUseFailure Hook)
- Name: post-tool-failure-{{ unix-coder }}
- Role: Implement the PostToolUseFailure hook for failed tool logging

- {{ reviewer }} (PermissionRequest Hook)
- Name: qa_tester
- Role: Conduct QA testing on the PostToolUseFailure hook by simulating tool failures and verifying logs [**main directive: generate and execute tests to validate the hook's functionality**]
- Agent Type: validator
- Resume: true

---
