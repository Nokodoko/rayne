# Plan: goNotebook RAG Integration for Feature Request Pipeline

## Overview

Integrate the Ultimate Go Notebook training material (`/home/n0ko/programming/go_projects/manuals/goNotebook/`) into the Claude agent sidecar's GitHub issue processing pipeline. When a new feature request arrives via GitHub webhook, the agent will query a dedicated Qdrant collection for relevant Go design principles, patterns, and code examples, then inject them into the Claude Code prompt so that implementations follow idiomatic Go practices from the Ardan Labs curriculum.

The goNotebook contains ~500KB of markdown (82 files) and ~420 Go source files spanning design philosophy, data semantics, interfaces, composition, error handling, concurrency, channels, testing, and profiling. This content will be chunked, embedded via Ollama (gemma:2b), and stored in a new `go_principles` Qdrant collection separate from the existing `rca_analyses` collection.

## Architecture Decision

**Approach: Sidecar-local ingestion script + on-demand RAG retrieval at prompt time**

The goNotebook directory lives on the host filesystem at `/home/n0ko/programming/go_projects/manuals/goNotebook/`, which is NOT inside the container. Two options exist:

1. **Host volume mount + startup ingestion** - Mount the goNotebook directory as a read-only volume into the claude-agent container, then run ingestion on server startup (with idempotency check).
2. **Pre-built ingestion script run from host** - A standalone Node.js script that reads from the host filesystem and calls the Qdrant/Ollama APIs directly (they are accessible on the k8s cluster network from the host via minikube service URLs or port-forwards).

**Decision: Option 1 (host volume mount + startup ingestion)** because:
- It keeps everything self-contained within the k8s deployment
- The init-on-startup pattern already exists (see `initQdrantCollection()` in agent-server.js line 1936-1939)
- The volume mount pattern is already used for `work-volume` which mounts the host rayne repo
- Idempotency is simple: check if collection exists AND has points; skip ingestion if populated
- Re-ingestion can be triggered via a new HTTP endpoint (`POST /go-principles/reingest`)

**Chunking strategy:**
- **Markdown files** (README.md): Split by `## ` or `### ` heading boundaries, keeping each section as a chunk with its heading hierarchy as metadata. These contain the design philosophy, guidelines, notes, and quotes that are most valuable.
- **Go source files** (*.go): Each file becomes one chunk. Files are short (20-100 lines typically) and self-contained examples. The file path provides topic context (e.g., `chap05/03_decoupling/main.go` -> topic: "decoupling").
- Chunk size target: 500-2000 tokens per chunk (the Ollama gemma:2b model handles this well).
- Each chunk gets metadata: `{ source: "goNotebook", category: "design|language|concurrency|testing|...", topic: "error_handling|composition|channels|...", file_path: "...", section_heading: "..." }`

**Retrieval strategy:**
- When `processGitHubIssue()` is called, extract keywords from the issue title + body
- Generate an embedding of the issue text using the same `generateEmbeddings()` function
- Search the `go_principles` collection for the top 5 most relevant chunks
- Inject the retrieved chunks into the prompt between the issue description and the implementation instructions

## Team Members

- Name: ingestion-engineer
- Role: Build the ingestion pipeline (chunking, embedding, Qdrant storage)
- Agent Type: unix-coder
- Resume: true

- Name: retrieval-engineer
- Role: Build the retrieval and prompt injection into processGitHubIssue
- Agent Type: unix-coder
- Resume: true

- Name: deployment-engineer
- Role: Handle Docker, k8s, and volume mount changes
- Agent Type: unix-coder
- Resume: true

## Step by Step Tasks

### 1. Add goNotebook Volume Mount to k8s Deployment

- **Task ID**: k8s-volume-mount
- **Depends On**: none
- **Assigned To**: deployment-engineer
- **Agent Type**: unix-coder
- **Parallel**: true (can run alongside task 2)

**Action Items:**
1. Edit `/home/n0ko/Portfolio/rayne/k8s/rayne-deployment.yaml`:
   - Add a new `hostPath` volume named `gonotebook-volume`:
     ```yaml
     - name: gonotebook-volume
       hostPath:
         path: /home/n0ko/programming/go_projects/manuals/goNotebook
         type: Directory
     ```
   - Add a `volumeMount` to the `claude-agent` container:
     ```yaml
     - name: gonotebook-volume
       mountPath: /app/gonotebook
       readOnly: true
     ```
2. Add an environment variable `GONOTEBOOK_PATH` to the claude-agent container:
   ```yaml
   - name: GONOTEBOOK_PATH
     value: "/app/gonotebook"
   ```
3. Verify the volume mount works: `kubectl apply -f k8s/rayne-deployment.yaml && kubectl exec -it <pod> -c claude-agent -- ls /app/gonotebook`

**Files Modified:**
- `k8s/rayne-deployment.yaml`

---

### 2. Implement goNotebook Chunking and Ingestion Functions

- **Task ID**: ingestion-functions
- **Depends On**: none
- **Assigned To**: ingestion-engineer
- **Agent Type**: unix-coder
- **Parallel**: true (can run alongside task 1)

**Action Items:**
1. Edit `/home/n0ko/Portfolio/rayne/docker/claude-agent/agent-server.js` to add the following constants and functions near the existing Qdrant code (after line ~17):
   ```javascript
   const GO_PRINCIPLES_COLLECTION = 'go_principles';
   const GONOTEBOOK_PATH = process.env.GONOTEBOOK_PATH || '/app/gonotebook';
   ```

2. Add function `initGoPrinciplesCollection()`:
   - Check if `go_principles` collection exists at Qdrant
   - If not, create it with same config as RCA: `{ vectors: { size: 2048, distance: 'Cosine' } }`
   - Return `{ exists: boolean, pointCount: number }` so caller knows if ingestion is needed

3. Add function `chunkMarkdownFile(filePath, category, topic)`:
   - Read the file content
   - Split by `## ` heading boundaries (keep heading with its content)
   - For each section, if it exceeds 3000 chars, further split by `### ` sub-headings
   - Return array of `{ text: string, metadata: { source: 'goNotebook', category, topic, file_path, section_heading } }`

4. Add function `chunkGoFile(filePath, category, topic)`:
   - Read the entire file as one chunk
   - Extract the directory name as sub-topic context
   - Prepend a context line: `// Go training example: {topic} - {subtopic}`
   - Return single-element array with same metadata structure

5. Add function `discoverGoNotebookFiles()`:
   - Walk the goNotebook directory tree recursively
   - Categorize files by their directory path:
     - `gotraining/topics/go/design/*` -> category: "design"
     - `gotraining/topics/go/concurrency/*` -> category: "concurrency"
     - `gotraining/topics/go/language/*` -> category: "language"
     - `gotraining/topics/go/testing/*` -> category: "testing"
     - `gotraining/topics/go/profiling/*` -> category: "profiling"
     - `gotraining/topics/go/packages/*` -> category: "packages"
     - `gotraining/topics/go/generics/*` -> category: "generics"
     - `ultimate_go_notebook/chap02/*` -> category: "language", topic: "syntax"
     - `ultimate_go_notebook/chap03/*` -> category: "language", topic: "data_semantics"
     - `ultimate_go_notebook/chap04/*` -> category: "language", topic: "decoupling"
     - `ultimate_go_notebook/chap05/*` -> category: "design", topic: from directory name
     - `ultimate_go_notebook/chap06/*` -> category: "concurrency"
     - `ultimate_go_notebook/chap07/*` -> category: "testing"
     - `ultimate_go_notebook/chap08/*` -> category: "testing", topic: "benchmarks"
     - `ultimate_go_notebook/chap09/*` -> category: "profiling"
     - `gotraining/topics/go/README.md` -> category: "philosophy" (THE most important file - design philosophy, guidelines)
   - Skip: `.git/`, `vendor/`, `grpcExample/`, `.DS_Store`, SSL certs, `.gitignore`
   - Return array of `{ filePath, fileType: 'md'|'go', category, topic }`

6. Add function `ingestGoNotebook()`:
   - Call `discoverGoNotebookFiles()`
   - For each file, call appropriate chunking function
   - For each chunk, call `generateEmbeddings(chunk.text)`
   - Batch upsert points to Qdrant (batch of 10 points per request to avoid overwhelming Ollama)
   - Add 500ms delay between batches for Ollama rate limiting
   - Log progress: `[GoNotebook] Ingested {n}/{total} chunks`
   - Return `{ chunksIngested: number, errors: number }`

**Priority content** (ingest the `gotraining/topics/go/README.md` design philosophy file FIRST as it is the single most valuable document at ~575 lines of concentrated Go design wisdom):
- Split this file into sections by `### ` headings
- Each section becomes its own chunk with high-signal metadata

**Files Modified:**
- `docker/claude-agent/agent-server.js`

---

### 3. Add Startup Ingestion with Idempotency

- **Task ID**: startup-ingestion
- **Depends On**: ingestion-functions
- **Assigned To**: ingestion-engineer
- **Agent Type**: unix-coder
- **Parallel**: false

**Action Items:**
1. Edit the server startup block in `agent-server.js` (line 1927-1940) to add goNotebook ingestion after the existing Qdrant init:
   ```javascript
   server.listen(PORT, async () => {
       // ... existing logging ...

       setTimeout(async () => {
           // Existing RCA collection init
           const initialized = await initQdrantCollection();
           console.log(`[Claude Agent] Qdrant RCA collection initialized: ${initialized}`);

           // GoNotebook collection init + conditional ingestion
           const goCollection = await initGoPrinciplesCollection();
           console.log(`[Claude Agent] Go principles collection: ${JSON.stringify(goCollection)}`);

           if (goCollection.exists && goCollection.pointCount > 0) {
               console.log(`[Claude Agent] Go principles already ingested (${goCollection.pointCount} points), skipping`);
           } else {
               console.log(`[Claude Agent] Starting goNotebook ingestion...`);
               const result = await ingestGoNotebook();
               console.log(`[Claude Agent] GoNotebook ingestion complete: ${JSON.stringify(result)}`);
           }
       }, 5000);
   });
   ```

2. The idempotency check uses point count: if the collection has >0 points, skip ingestion. This is simple and sufficient since the goNotebook content is static.

**Files Modified:**
- `docker/claude-agent/agent-server.js`

---

### 4. Add Re-ingestion HTTP Endpoint

- **Task ID**: reingest-endpoint
- **Depends On**: ingestion-functions
- **Assigned To**: ingestion-engineer
- **Agent Type**: unix-coder
- **Parallel**: true (can run alongside task 5)

**Action Items:**
1. Add a new route in agent-server.js route handler (around line 1843):
   ```javascript
   // GoNotebook re-ingestion endpoint
   if (url.pathname === '/go-principles/reingest' && req.method === 'POST') {
       try {
           // Delete existing collection
           await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}`, 'DELETE');
           // Re-create and ingest
           await initGoPrinciplesCollection();
           const result = await ingestGoNotebook();
           sendJson(res, 200, { status: 'reingested', ...result });
       } catch (err) {
           sendJson(res, 500, { error: err.message });
       }
       return;
   }
   ```

2. Add a GET endpoint for inspection:
   ```javascript
   if (url.pathname === '/go-principles/stats' && req.method === 'GET') {
       // Return collection info and sample points
   }
   ```

**Files Modified:**
- `docker/claude-agent/agent-server.js`

---

### 5. Implement Retrieval Function for Go Principles

- **Task ID**: retrieval-function
- **Depends On**: ingestion-functions
- **Assigned To**: retrieval-engineer
- **Agent Type**: unix-coder
- **Parallel**: true (can run alongside task 4)

**Action Items:**
1. Add function `searchGoPrinciples(queryText, limit = 5)` in agent-server.js:
   - Call `generateEmbeddings(queryText)` to get the embedding
   - Search the `go_principles` collection with the embedding
   - Optionally filter by category if the issue mentions specific topics (e.g., if "concurrency" appears in the issue, add a Qdrant filter `{ must: [{ key: "category", match: { value: "concurrency" } }] }`)
   - Return array of `{ text, score, metadata }` sorted by relevance score
   - If embedding generation fails or collection is empty, return empty array (graceful degradation)

2. Add function `formatGoPrinciplesContext(results)`:
   - Take the search results and format them into a markdown section for prompt injection
   - Format:
     ```
     ## Go Design Principles (Retrieved from Ultimate Go Notebook)

     The following Go design principles and patterns are relevant to this feature request.
     Follow these guidelines when implementing:

     ### {metadata.topic} ({metadata.category})
     {text}

     ---
     ```
   - Limit total injected context to ~4000 chars to avoid bloating the prompt
   - Prioritize chunks with higher similarity scores

**Files Modified:**
- `docker/claude-agent/agent-server.js`

---

### 6. Inject Retrieved Principles into processGitHubIssue Prompt

- **Task ID**: prompt-injection
- **Depends On**: retrieval-function
- **Assigned To**: retrieval-engineer
- **Agent Type**: unix-coder
- **Parallel**: false

**Action Items:**
1. Modify `processGitHubIssue()` function (line 924-1014) to add Go principles retrieval:
   - After building `pastIssuesContext` (line 945), add:
     ```javascript
     // Retrieve relevant Go design principles
     let goPrinciplesContext = '';
     try {
         const queryText = `${safeTitle} ${safeBody}`.substring(0, 1000);
         const principles = await searchGoPrinciples(queryText, 5);
         if (principles.length > 0) {
             goPrinciplesContext = formatGoPrinciplesContext(principles);
         }
     } catch (err) {
         console.error(`[GitHub] Failed to retrieve Go principles: ${err.message}`);
         // Continue without principles - graceful degradation
     }
     ```

2. Inject `goPrinciplesContext` into the prompt template, between the issue body and the implementation instructions. Modify the prompt string (line 947-995):
   ```javascript
   const prompt = `You are implementing a feature request...

   ## Issue: ${safeTitle}

   ${safeBody}
   ${pastIssuesContext}
   ${goPrinciplesContext}
   ## CRITICAL: Duplicate Detection
   ...
   ## Instructions (only if NOT a duplicate)

   1. Read the CLAUDE.md and any agentic_instructions.md files...
   2. **IMPORTANT**: Follow the Go design principles provided above. Specifically:
      - Use value/pointer semantics consistently
      - Design interfaces based on behavior, not data
      - Handle errors as part of the main code path
      - Keep interfaces small (1-2 methods)
      - Write code that is readable by the average developer
   3. Explore the existing codebase...
   ...`;
   ```

3. The key change is adding instruction #2 that explicitly tells Claude to follow the retrieved principles. This bridges the gap between having the context and actually using it.

**Files Modified:**
- `docker/claude-agent/agent-server.js`

---

### 7. Update Dockerfile for goNotebook Support

- **Task ID**: dockerfile-update
- **Depends On**: k8s-volume-mount
- **Assigned To**: deployment-engineer
- **Agent Type**: unix-coder
- **Parallel**: true

**Action Items:**
1. Edit `/home/n0ko/Portfolio/rayne/docker/claude-agent/Dockerfile`:
   - No new dependencies needed (all existing: `fs`, `path`, `http` are sufficient)
   - Add a comment documenting the goNotebook volume mount expectation:
     ```dockerfile
     # GoNotebook training material is mounted at runtime via k8s hostPath volume
     # Expected at: /app/gonotebook (set via GONOTEBOOK_PATH env var)
     ```
   - Ensure the `/app/gonotebook` directory is accessible by the `node` user (it will be since it is a read-only mount)

2. No changes to the Docker build itself -- the goNotebook content is mounted at runtime, not baked into the image. This is intentional so the image stays lean and the content can be updated without rebuilding.

**Files Modified:**
- `docker/claude-agent/Dockerfile` (comment only)

---

### 8. Integration Testing

- **Task ID**: integration-testing
- **Depends On**: prompt-injection, k8s-volume-mount, startup-ingestion, reingest-endpoint
- **Assigned To**: retrieval-engineer
- **Agent Type**: unix-coder
- **Parallel**: false

**Action Items:**
1. **Local verification** (before deploying to k8s):
   - Set `GONOTEBOOK_PATH` to the host path and run agent-server.js locally
   - Verify ingestion completes and logs chunk counts
   - Call `GET /go-principles/stats` to verify collection has points
   - Call `POST /go-principles/reingest` to verify re-ingestion works

2. **Retrieval quality check**:
   - Test search with various query strings:
     - "Add a new REST endpoint for managing user profiles" -> should retrieve interface design, handler patterns
     - "Implement concurrent batch processing for webhooks" -> should retrieve concurrency patterns, channel design
     - "Add error handling for database connection failures" -> should retrieve error handling design
   - Verify the top results are actually relevant

3. **Full pipeline test**:
   - Deploy to minikube: rebuild claude-agent image, `kubectl apply`, restart deployment
   - Create a test GitHub issue (or use the `/github/process-issue` endpoint directly)
   - Verify the Claude Code prompt includes the Go principles section
   - Check that the resulting implementation follows the retrieved patterns

4. **Graceful degradation test**:
   - Delete the `go_principles` collection from Qdrant
   - Process an issue and verify it still works (just without Go principles context)
   - Stop Ollama and verify ingestion failure is handled gracefully

**Verification commands:**
```bash
# Check collection exists
curl http://localhost:6333/collections/go_principles

# Check point count
curl http://localhost:6333/collections/go_principles/points/count

# Test search via agent endpoint
curl -X GET http://localhost:9000/go-principles/stats

# Force re-ingestion
curl -X POST http://localhost:9000/go-principles/reingest
```

---

### 9. Documentation Update

- **Task ID**: documentation-update
- **Depends On**: integration-testing
- **Assigned To**: deployment-engineer
- **Agent Type**: unix-coder
- **Parallel**: false

**Action Items:**
1. Update the `CLAUDE.md` in the rayne project root to document the goNotebook RAG integration:
   - Add a new section under "AI-Powered Root Cause Analysis" describing the Go principles RAG
   - Document the new agent endpoints (`/go-principles/stats`, `/go-principles/reingest`)
   - Document the `GONOTEBOOK_PATH` environment variable

2. Update `docker/claude-agent/agentic_instructions.md` if relevant to note that Go design principles are now automatically injected.

**Files Modified:**
- `CLAUDE.md` (rayne project root)
- `docker/claude-agent/agentic_instructions.md`

---

## Summary of All Modified Files

| File | Change Type | Task |
|------|------------|------|
| `docker/claude-agent/agent-server.js` | Major: new functions, modified startup, new routes, modified prompt | Tasks 2, 3, 4, 5, 6 |
| `k8s/rayne-deployment.yaml` | Minor: new volume + volumeMount + env var | Task 1 |
| `docker/claude-agent/Dockerfile` | Trivial: comment only | Task 7 |
| `CLAUDE.md` | Minor: documentation | Task 9 |
| `docker/claude-agent/agentic_instructions.md` | Minor: documentation | Task 9 |

## New Files

None. All changes are modifications to existing files. The ingestion logic lives entirely within `agent-server.js`, following the existing pattern where all Qdrant/Ollama integration is in that single file.

## Risk Mitigation

1. **Ollama rate limiting during ingestion**: Batch embeddings with 500ms delays. The ~500 chunks will take approximately 4-5 minutes to ingest. This only happens once (or on manual re-ingestion).

2. **Embedding quality**: The gemma:2b model produces 2048-dimensional embeddings. While not the most powerful embedding model, it is already proven to work for the RCA similarity search. If retrieval quality is poor, consider upgrading to a dedicated embedding model like `nomic-embed-text` in a future iteration.

3. **Prompt size bloat**: Cap the injected Go principles context at ~4000 characters (roughly 5 chunks). This adds acceptable overhead to the prompt without overwhelming the context window.

4. **goNotebook directory unavailable**: If the volume mount fails or the directory is empty, ingestion gracefully skips and `searchGoPrinciples()` returns an empty array. The pipeline continues to work exactly as it does today.

5. **Qdrant storage**: Adding ~500 vectors of 2048 dimensions requires minimal additional storage (~4MB). Well within the existing 5Gi PVC allocation.

## Execution Order (Critical Path)

```
Tasks 1 + 2 (parallel)
    |        |
    v        v
  Task 7   Task 3 --> Task 4 (parallel with 5)
              |        Task 5
              v          |
           Task 6 <------+
              |
              v
           Task 8
              |
              v
           Task 9
```

Total estimated effort: 4-6 hours of implementation across 3 parallel agents.
