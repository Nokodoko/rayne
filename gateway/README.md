# Monty Gateway (base)

Lightweight FastAPI gateway that bridges WebSocket connections from the
frontend `chat.js` to a local Ollama instance.  No multi-agent routing,
no RAG, no gRPC -- just a direct Ollama-to-WebSocket streaming bridge.

## Quick start

```bash
# Install dependencies
pip install -r requirements.txt

# Make sure Ollama is running with llama3.2
ollama pull llama3.2:latest
ollama serve            # default :11434

# Start the gateway
python main.py          # default :8001
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_PORT` | `8001` | Port the gateway listens on |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama base URL |
| `OLLAMA_MODEL` | `llama3.2:latest` | Model to use for generation |

## WebSocket protocol

Connect to `ws://HOST:PORT/chat/ws` (or the alias `/ws/chat`).

### Client sends

```json
{"message": "Hello", "conversation_id": "optional-uuid"}
```

If `conversation_id` is omitted the server generates one.

### Server streams back

```json
{"task_id": "...", "event_type": "content",   "content": "token", "conversation_id": "...", "is_complete": false}
{"task_id": "...", "event_type": "completed", "content": "Task completed", "conversation_id": "..."}
{"task_id": "...", "event_type": "error",     "error_message": "...", "conversation_id": "..."}
```

Every event includes `conversation_id` (fixing a bug in the original Monty
gateway where it was omitted).

## Health check

```bash
curl http://localhost:8001/health
# {"status":"healthy"}
```
