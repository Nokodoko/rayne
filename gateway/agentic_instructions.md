# agentic_instructions.md

## Purpose
Lightweight FastAPI WebSocket gateway for the Monty chatbot. Bridges frontend chat.js WebSocket connections to a local Ollama instance running llama3.2:latest with streaming token delivery and Datadog LLM Observability integration.

## Technology
Python, FastAPI, WebSocket, httpx (async streaming), Datadog ddtrace (patch, LLMObs), uvicorn

## Contents
- `main.py` -- Complete gateway: configuration, system prompt (CHRIS_PROFILE), Ollama streaming helper, FastAPI app with WebSocket endpoints, conversation history management
- `start-gateway.sh` -- Shell script to start the gateway

## Key Functions
- `stream_ollama(conversation_id, user_message, task_id, websocket)` -- Sends prompt to Ollama, streams response tokens back over WebSocket as content events, sends completed event when done
- `_handle_chat(websocket)` -- Shared WebSocket handler: accepts connection, parses JSON messages, dispatches to stream_ollama
- `chat_ws(websocket)` -- Primary WebSocket endpoint at /chat/ws
- `ws_chat(websocket)` -- Alias endpoint at /ws/chat
- `health()` -- GET /health returns status
- `lifespan(app)` -- Startup/shutdown lifecycle: enables LLMObs, clears conversations on shutdown

## Data Types
- `conversations` -- dict[str, list[dict[str, str]]]: in-memory per-connection conversation history
- WebSocket event types: "content" (streaming tokens), "completed" (generation done), "error" (failures)
- Configuration: GATEWAY_PORT (8001), OLLAMA_HOST (localhost:11434), OLLAMA_MODEL (llama3.2:latest)

## Logging
Uses `print()` for startup/shutdown messages

## CRUD Entry Points
- **Create**: N/A (stateless gateway)
- **Read**: Connect to ws://host:8001/chat/ws or /ws/chat
- **Update**: Modify SYSTEM_PROMPT for different chatbot personality, OLLAMA_MODEL for different LLM
- **Delete**: N/A

## Style Guide
- Async/await throughout with httpx.AsyncClient for streaming
- Datadog LLMObs span management: LLMObs.llm() + LLMObs.annotate() in try/finally
- CORS middleware allows all origins
- Representative snippet:

```python
async def stream_ollama(conversation_id: str, user_message: str, task_id: str, websocket: WebSocket) -> None:
    history = conversations.setdefault(conversation_id, [])
    history.append({"role": "user", "content": user_message})

    payload = {"model": OLLAMA_MODEL, "prompt": prompt, "system": SYSTEM_PROMPT, "stream": True}

    span = LLMObs.llm(model_name=OLLAMA_MODEL, model_provider="ollama", name="ollama.generate")
    async with httpx.AsyncClient(timeout=httpx.Timeout(300.0)) as client:
        async with client.stream("POST", f"{OLLAMA_HOST}/api/generate", json=payload) as response:
            async for line in response.aiter_lines():
                chunk = json.loads(line)
                token = chunk.get("response", "")
                if token:
                    await websocket.send_json({"task_id": task_id, "event_type": "content", "content": token})
```
