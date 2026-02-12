# Classify

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

### Classification_Types:

**PLATFORM**: [
"github",
"gitlab",
"bitbucket",
"datadog",
"aws",
"gcp",
"azure",
"slack",
"jira",
"confluence"
]

**Tecch Stack**: [
"javascript",
"typescript",
"python",
"go",
"java",
"ruby",
"php",
"c#",
"c++",
"node",
"react",
"angular","vue",
"django",
"flask","spring",
"rails","laravel",
".net"
]

**Error Types**[
"syntax error",
"runtime error",
"dependency error",
"configuration error",
"permission error",
"network error",
"performance issue",
"security vulnerability",
"infrastucture error"
"infrastructure resource constaint"
]

---

# Arguments: Webhook Payload

Raw input: `$ARGUMENTS`

# Step 1: Spawn the supervisor

## Workflow

1. Immediately Launch a single **general-purpose** sub-agent as the {{ supervisor }}. Pass it the ENTIRE contents of this command file plus the raw `$ARGUMENTS` string. The supervisor self-bootstraps by reading the command prompt to understand its mission. The base orchestrator waits for it to return.

2. Based on webhook payloads, classify the vital types found in the webhook payload to help guide the orchestration of the workflow.

## Step 2: Instructions

1. Analyze the webhook payload to identify key information such as the platform, tech stack, and error types involved.
2. Classify the webhook payload based on the identified information, categorizing it into the appropriate platform, tech stack, and error type using the {{ Classification_Types }} dictionary.
3. Use the classification results to guide the orchestration of the workflow, ensuring that the appropriate agents and tools are utilized for the specific context of the webhook payload.

#### Task Management Tools

note these are examples! The titles of each task and the output should be the same. The values of each key will differ based on the specific task and context.

`````typescript
    TaskCreate({
      subject: "Implement user authentication",
      description: "Create login/logout endpoints with JWT tokens. See specs/auth-plan.md for details.",
      activeForm: "Implementing authentication"  // Shows in UI spinner when in_progress
    })
    // Returns: taskId (e.g., "1")
    ```

**TaskUpdate** – Update task status, assignment, or dependencies:
```typescript
    TaskUpdate({
      taskId: "1",
      status: "in_progress",  // pending → in_progress → completed
      owner: "{{ unix-coder }}-auth"   // Assign to specific team member
    })
    ```

**TaskList** – View all tasks and their status:
```typescript
    TaskList({})
    // Returns: Array of tasks with id, subject, status, owner, blockedBy
    ```

**TaskGet** – Get full details of a specific task:
```typescript
    TaskGet({ taskId: "1" })
    // Returns: Full task including description
    ````
`````

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

---

Based on webhook payloads, user interactions, or other triggers, you may need to implement specific hooks that require coordination between agents. For example, if you have a `SessionEnd` hook that needs to perform cleanup tasks and log session data, you would assign one {{ unix-coder }} to implement the hook's functionality and a {{ reviewer }} to validate it.

Recursively traverse directories and generate context-aware `agentic_instructions.md` documentation for each directory. This documentation serves as a per-directory index for navigating, understanding, and performing CRUD operations within the codebase.

## Arguments

Raw input: `$ARGUMENTS`

Parse the following from the raw input above. Any unrecognized positional argument is the **target directory**.

| Flag       | Short | Default    | Description                                  |
| ---------- | ----- | ---------- | -------------------------------------------- |
| `--depth`  | `-d`  | infinite   | Max directory traversal depth                |
| `--output` | `-o`  | `markdown` | Output format: `markdown`, `html`, or `json` |

Special hooks:
| Keyword | Description |
|---------|-------------|
| `clear git` | Remove `.git/`, re-initialize repo, and commit all files (see Hook: clear git below) |

- If no target directory is provided, use the current working directory.
- If `--help` is passed, print the following and stop:

````

/dir_instructions [flags] [hooks] [target_directory]

Flags:
--depth, -d <n> Max directory traversal depth (default: infinite)
--output, -o <fmt> Output format: markdown | html | json (default: markdown)
--help Show this help message

Hooks (keyword triggers — include anywhere in arguments):
clear git Destroy .git/, re-init repo, commit all files, then
proceed with doc generation. Requires confirmation.
(see "Hook: clear git" section for details)

Architecture:
Base orchestrator → Supervisor (self-bootstrapping) → Worker agents
(base context contains only raw args + final summary)

The supervisor reads the full command prompt that created it,
bootstraps its own mission, and coordinates all phases autonomously.

Phases (planned and executed by supervisor):
1 Discovery — Explore agent catalogs eligible directories
2 Parallel doc generation — one unix-coder per directory
3 Root index with Key Abstractions — unix-coder reads docs, not source
4 Verification — Explore agent confirms full coverage
5 Summary returned to base orchestrator, advise context clearing

Examples:
/dir_instructions ./src
/dir_instructions -d 2 -o json ./lib
/dir_instructions --depth 3
/dir_instructions clear git ./myproject

```

## Hook: clear git

If the raw input contains the string `clear git`, execute this hook **before** any other phase:

1. **Confirm with the user.** Print: `This will permanently delete .git/ in the target directory, destroying all git history. A fresh repo will be initialized and all files committed. Proceed? (yes/no)` — wait for explicit "yes" before continuing. If the user says anything other than "yes", abort the entire command.
2. Run `rm -rf <target_dir>/.git`
3. Run `git init` in the target directory
4. Run `git add -A && git commit -m "Initial commit (history cleared by dir_instructions)"` in the target directory
5. Strip `clear git` from the arguments and continue to the phases below with the remaining flags/target.

## Instructions

You are the **base orchestrator** — an ultra-thin launcher. Your ONLY job is:

1. Parse `$ARGUMENTS` for `--help` (print help and stop if present)
2. Handle any hooks that require user confirmation (destructive actions must stay in base for user interaction)
3. Spawn a **supervisor** agent and pass it the ENTIRE raw command prompt (everything from the first line of this file through the Constraints section) plus the parsed `$ARGUMENTS` string
4. Wait for the supervisor to return a summary, print it, and advise context clearing

You must NEVER read files, traverse directories, generate documentation, process phase definitions, interpret output format specs, or plan anything. The command prompt content, all phase definitions, exclusion rules, and output format specifications exist solely for the supervisor to consume — they do not belong in your context.

### Step 1: Parse arguments and handle hooks

Parse flags, hooks, and target directory from `$ARGUMENTS` as described above. If `--help` is present, print the help text and stop. If `clear git` is present, execute that hook first (it runs in the base orchestrator since it requires user confirmation). Then proceed to Step 2.

### Step 2: Spawn the supervisor

Launch a single **general-purpose** sub-agent as the supervisor. Pass it the ENTIRE contents of this command file plus the raw `$ARGUMENTS` string. The supervisor self-bootstraps by reading the command prompt to understand its mission. The base orchestrator waits for it to return.

**The base orchestrator's context is now completely protected.** It contains only: the raw `$ARGUMENTS` string, any hook interactions, and the final summary. No directory listings, file contents, code, phase definitions, or sub-agent outputs ever enter its context.

---

## Supervisor Prompt

You have been spawned as the supervisor for `dir_instructions`. The command prompt that created you contains your full mission. Read it, understand your phases, output format, exclusion rules, and constraints, then proceed autonomously.

You coordinate all discovery, documentation, verification, and reporting. Do NOT write code or documentation yourself — delegate everything to worker agents. You create your own plans and prompts for workers based on the command prompt you received.

### Phase 1: Discovery (delegated to Explore worker agent)

Launch a single **Explore** worker agent with the following prompt. You receive back only the final directory list — no file contents, no code, no traversal details.

Explore agent prompt:

> Starting from `<target_directory>`, recursively list all subdirectories (respecting `--depth` if set). For each directory, determine whether it should be **included** or **excluded**.
>
> **Exclude** directories matching any of these patterns:
>
> Vendored / dependency directories:
>
> - `node_modules/`, `vendor/`, `third_party/`, `third-party/`, `external/`, `deps/`
> - `.venv/`, `venv/`, `env/`, `__pypackages__/`, `site-packages/`
> - `Pods/`, `Carthage/`, `.bundle/`
> - `go/pkg/mod/`, `target/debug/`, `target/release/`
> - `.git/`, `.hg/`, `.svn/`
> - Any directory that is a package manager install target (check for lock files, `.package` markers, or manifest-declared dependency paths)
>
> Artifact / asset directories with no parseable source code:
>
> - `img/`, `images/`, `assets/`, `static/`, `media/`, `icons/`, `sprites/`
> - `fonts/`, `dist/`, `build/`, `out/`, `bin/`, `obj/`
> - Any directory containing only binary files (images, compiled outputs, fonts, audio, video)
>
> When unsure, check for telltale signs: a license file from another project, a `package.json`/`go.mod`/`Cargo.toml` that doesn't belong to the root project, hundreds of subdirectories from different authors, or a directory where every file has a binary/media extension.
>
> **Return only** two lists:
>
> 1. **Included directories** — absolute paths, one per line
> 2. **Excluded directories** — absolute paths with reason for exclusion, one per line
>
> Do not return file contents, code snippets, or any other information.

Use only the **Included directories** list to drive Phase 2.

### Phase 2: Parallel Documentation Generation

Launch **one unix-coder worker agent per included directory**. Each worker is scoped to a single directory and only reads files within it. Do NOT group or batch directories.

Once a worker completes, hand its output to an **Explore** agent for review. Feed review feedback back to the unix-coder until the documentation is accurate and complete.

All workers must be launched concurrently in a single fan-out. Wait for all to finish before proceeding to Phase 3.

### Phase 3: Root Index

After all per-directory files are written, launch a **unix-coder** worker agent to generate a single `agentic_instructions.md` at the root of the target directory. This agent reads the per-directory `agentic_instructions.md` files (not the source code) and consolidates them into a routing index — enough context for an orchestrator to dispatch agents to the correct directory for any CRUD operation.

The root file must include a **Key Abstractions** section. This section maps the codebase's core domain concepts to their concrete implementations so that agents can jump directly to the right location without exploratory searches. For each abstraction, include:

- **Name** — The domain concept (e.g. `User`, `AuthSession`, `Pipeline`, `EventBus`)
- **Type** — What it is (class, interface, module, service, pattern)
- **Location** — File path(s) where it's defined
- **Relationships** — What it depends on or what depends on it (e.g. `User` → `AuthSession` → `TokenStore`)
- **CRUD routing** — Which directory/file to target for create, read, update, or delete operations on this abstraction

Group abstractions by domain area (e.g. Auth, Data, Messaging) rather than by directory.

### Phase 4: Verification

Run a coverage check to ensure no eligible directories were missed:

1. Launch an **Explore** agent to independently list every non-excluded directory under the target (applying the same exclusion rules from Phase 1).
2. Compare that list against the set of directories that actually received an `agentic_instructions.md` file.
3. If any directories are missing, report them and generate docs for them (repeating Phase 2 for the missing directories) or explicitly skip them with documented reasons.
4. Do NOT proceed until every eligible directory is accounted for.

### Phase 5: Return Summary to Base Orchestrator

Return **only** the following to the base orchestrator (the ultra-thin launcher that spawned you):

- Total directories documented
- Total files generated
- Directories skipped with reasons
- Any errors or warnings

Do NOT return file contents, directory listings, code, or any other bulk data. The base orchestrator will print this summary and advise context clearing.

---

## Per-Directory `agentic_instructions.md` Format

Each file must include:

1. **Purpose** — What this directory is for, in 1-2 sentences
2. **Technology** — Language, framework, stack
3. **Contents** — List of files and subdirectories with brief descriptions
4. **Key Functions** — Major functions/methods, their signatures, and return types
5. **Data Types** — Structs, interfaces, classes, enums with properties and methods
6. **Logging** — Logging style used in this directory
7. **CRUD Entry Points** — How to create, read, update, or delete content in this directory
8. **Style Guide** — A short reference showing the prevailing coding style in this directory so that any newly generated code matches it. Include:
   - Naming conventions (camelCase, snake_case, PascalCase for files/variables/functions/types)
   - Import/module ordering and grouping
   - Error handling pattern (try/catch, Result types, error codes, etc.)
   - A **representative code snippet** (10–20 lines) copied verbatim from the directory that best demonstrates the typical structure, formatting, and idioms used. Choose a snippet that shows function signatures, control flow, and naming in action.

## Context Clearing

This command is designed to be the first step in a larger meta-prompt workflow. After the base orchestrator receives the supervisor's summary:

1. Print the summary to the user.
2. Output: `All agentic_instructions.md files generated. Context from this command should be cleared before proceeding to the next step. Use /clear or start a new conversation to free the context window.`

The generated `agentic_instructions.md` files are now the sole source of truth — future agents read those files instead of relying on any orchestrator's context.

## Constraints

- The **base orchestrator** is an ultra-thin launcher — it ONLY parses args, handles hooks requiring confirmation, spawns the supervisor, and receives the summary. It must NEVER read files, traverse directories, generate documentation, process phase definitions, or plan anything.
- The **supervisor** self-bootstraps from the command prompt — it reads its own mission, phases, output format, and exclusion rules from the prompt that created it. It creates its own plans and prompts for worker agents. It must NEVER write code or docs itself — it only coordinates workers.
- Parallelize worker agents wherever possible
- Continue the unix-coder / review cycle until each file is complete and accurate

<!-- Additional documentation: ~/.claude/commands/docs/dir_instructions.md -->

```

```

```
````

```

```
