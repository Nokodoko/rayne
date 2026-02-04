"""
Lightweight FastAPI gateway for the Monty chatbot.

Bridges WebSocket connections from the frontend chat.js to a local Ollama
instance running llama3.2:latest. No multi-agent orchestration, no RAG,
no gRPC -- just a direct Ollama-to-WebSocket streaming bridge.
"""

import asyncio
import json
import os
import uuid
from contextlib import asynccontextmanager

import httpx
import uvicorn
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

GATEWAY_PORT = int(os.getenv("GATEWAY_PORT", "8001"))
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "http://localhost:11434")
OLLAMA_MODEL = os.getenv("OLLAMA_MODEL", "llama3.2:latest")

# ---------------------------------------------------------------------------
# System prompt (CHRIS_PROFILE)
# ---------------------------------------------------------------------------

SYSTEM_PROMPT = """\
You are an AI assistant for Chris Montgomery's personal website (n0ko.com).
Your purpose is to answer questions about Chris - his professional background, skills, experience, and projects.

## About Chris Montgomery

Chris is a Platform Engineer and Observability SME based in Dedham, Massachusetts.
Contact: cmonty614@gmail.com | LinkedIn: linkedin.com/in/cmonty614

### Current Role
**Site Reliability Engineer at ECCO Select** (September 2023 - Present)
Observability Specialist leading large data centers using Google's four golden metrics (latency, traffic, errors, saturation). Expert in Datadog platform including APM, RUM, Synthetic Testing, and Infrastructure Monitoring. Specializes in CI/CD optimization, Terraform, and Python scripting.

### Top Skills
Kubernetes, Amazon EKS, Helm Charts, Datadog, AWS, Docker, Python, Terraform, Go (Golang), Linux Administration

### Certifications
- Datadog Certified: Datadog Fundamentals
- Datadog Certified: Log Management Fundamentals
- Datadog Certified: APM & Distributed Tracing Fundamentals

### Previous Experience
- **Capacity** - DevOps Engineer II (Dec 2021 - Aug 2023): Kubernetes administration (100+ services on EKS), AWS administration, GitLab, Jira, RabbitMQ monitoring, Datadog integration
- **CJT Technology** - DevOps Engineer (Feb 2018 - Dec 2021): Containerized applications with Go/Java/Postgres on Kubernetes, CI/CD, Ansible automation
- **Hub Recruiting** - Sr. Recruiter (Sep 2019 - Apr 2020): Full-life cycle recruitment
- **Wayfair** - Talent Acquisition (Nov 2018 - Apr 2019): Recruited Full-Stack developers
- **State Street** - Assistant Vice President (Feb 2017 - Nov 2018): Professional Development Program Designer
- **United States Air Force** - Aerospace and Propulsion (2009-2012): TF-34 Jet Engine technician
- **Barnum Financial Group** - Registered Representative (2008-2010): Series 6, 63, Life and Health licenses

### Education
- Nichols College: BA Psychology (2001-2005)
- MIT Professional Education: Cloud & DevOps: Continuous Transformation
- Udemy: Linux, Docker, Go, Protocol Buffers

### Key Projects at Capacity
- Datadog POC and Integration
- Nginx sidecar removal
- LastPass security breach remediation
- Helm chart standardization
- Docker base image rebuild (compiled from source, reduced attack vectors)

Be friendly and professional. If asked about unrelated topics, politely redirect to learning about Chris and his work.
You do NOT have access to any tools or the ability to execute commands."""

# ---------------------------------------------------------------------------
# Per-connection conversation history (in-memory, lost on restart)
# ---------------------------------------------------------------------------

conversations: dict[str, list[dict[str, str]]] = {}

# ---------------------------------------------------------------------------
# Ollama streaming helper
# ---------------------------------------------------------------------------


async def stream_ollama(
    conversation_id: str,
    user_message: str,
    task_id: str,
    websocket: WebSocket,
) -> None:
    """Send a prompt to Ollama and stream the response tokens back over the
    WebSocket as ``content`` events.  Sends a ``completed`` event when the
    generation finishes."""

    # Initialise or retrieve conversation history
    if conversation_id not in conversations:
        conversations[conversation_id] = []

    history = conversations[conversation_id]
    history.append({"role": "user", "content": user_message})

    # Build the full prompt from history with the system prompt prepended.
    # Ollama's /api/generate accepts a single ``prompt`` string (and an
    # optional ``system`` field), so we concatenate the conversation turns.
    prompt_parts: list[str] = []
    for turn in history:
        role_label = "User" if turn["role"] == "user" else "Assistant"
        prompt_parts.append(f"{role_label}: {turn['content']}")
    prompt_parts.append("Assistant:")
    prompt = "\n".join(prompt_parts)

    payload = {
        "model": OLLAMA_MODEL,
        "prompt": prompt,
        "system": SYSTEM_PROMPT,
        "stream": True,
    }

    full_response = ""

    try:
        async with httpx.AsyncClient(timeout=httpx.Timeout(300.0)) as client:
            async with client.stream(
                "POST",
                f"{OLLAMA_HOST}/api/generate",
                json=payload,
            ) as response:
                response.raise_for_status()

                async for line in response.aiter_lines():
                    if not line:
                        continue

                    try:
                        chunk = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    token = chunk.get("response", "")
                    if token:
                        full_response += token
                        await websocket.send_json(
                            {
                                "task_id": task_id,
                                "event_type": "content",
                                "content": token,
                                "conversation_id": conversation_id,
                                "is_complete": False,
                            }
                        )

                    # Ollama signals completion with {"done": true}
                    if chunk.get("done"):
                        break

        # Store assistant reply in history
        history.append({"role": "assistant", "content": full_response})

        # Send completed event
        await websocket.send_json(
            {
                "task_id": task_id,
                "event_type": "completed",
                "content": "Task completed",
                "conversation_id": conversation_id,
            }
        )

    except httpx.HTTPStatusError as exc:
        await websocket.send_json(
            {
                "task_id": task_id,
                "event_type": "error",
                "error_message": f"Ollama returned HTTP {exc.response.status_code}",
                "conversation_id": conversation_id,
            }
        )
    except httpx.ConnectError:
        await websocket.send_json(
            {
                "task_id": task_id,
                "event_type": "error",
                "error_message": f"Cannot connect to Ollama at {OLLAMA_HOST}",
                "conversation_id": conversation_id,
            }
        )
    except Exception as exc:  # noqa: BLE001
        await websocket.send_json(
            {
                "task_id": task_id,
                "event_type": "error",
                "error_message": str(exc),
                "conversation_id": conversation_id,
            }
        )


# ---------------------------------------------------------------------------
# FastAPI application
# ---------------------------------------------------------------------------


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup / shutdown lifecycle."""
    print(f"Monty gateway starting on :{GATEWAY_PORT}")
    print(f"Ollama: {OLLAMA_HOST}  Model: {OLLAMA_MODEL}")
    yield
    # Cleanup conversation memory
    conversations.clear()
    print("Monty gateway stopped")


app = FastAPI(title="Monty Gateway", lifespan=lifespan)

# CORS -- allow everything so the frontend can connect from any origin.
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ---------------------------------------------------------------------------
# Health endpoint
# ---------------------------------------------------------------------------


@app.get("/health")
async def health():
    return {"status": "healthy"}


# ---------------------------------------------------------------------------
# WebSocket endpoints
# ---------------------------------------------------------------------------


async def _handle_chat(websocket: WebSocket) -> None:
    """Shared handler for both /chat/ws and /ws/chat."""
    await websocket.accept()

    try:
        while True:
            raw = await websocket.receive_text()

            try:
                data = json.loads(raw)
            except json.JSONDecodeError:
                await websocket.send_json(
                    {
                        "task_id": str(uuid.uuid4()),
                        "event_type": "error",
                        "error_message": "Invalid JSON",
                        "conversation_id": None,
                    }
                )
                continue

            message = data.get("message", "").strip()
            conversation_id = data.get("conversation_id") or str(uuid.uuid4())
            task_id = str(uuid.uuid4())

            if not message:
                await websocket.send_json(
                    {
                        "task_id": task_id,
                        "event_type": "error",
                        "error_message": "Empty message",
                        "conversation_id": conversation_id,
                    }
                )
                continue

            await stream_ollama(conversation_id, message, task_id, websocket)

    except WebSocketDisconnect:
        pass


@app.websocket("/chat/ws")
async def chat_ws(websocket: WebSocket):
    """Primary WebSocket endpoint (used by chat.js)."""
    await _handle_chat(websocket)


@app.websocket("/ws/chat")
async def ws_chat(websocket: WebSocket):
    """Alias endpoint for convenience."""
    await _handle_chat(websocket)


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=GATEWAY_PORT,
        reload=False,
    )
