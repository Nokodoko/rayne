"""
Datadog LLM Observability wrapper for the Monty AI agent's Ollama client.

This module provides instrumentation for Ollama chat completions via
ddtrace's LLM Observability (LLMObs) API. It wraps the OllamaClient
methods to automatically track:

  - Model name
  - Prompt and completion token counts
  - Input/output text
  - Latency (via span timing)
  - Errors and exceptions

Usage:
    # Import and enable LLM Observability at application startup
    from ddtrace_llm_wrapper import enable_llm_monitoring, instrument_ollama_client

    enable_llm_monitoring()

    # Wrap an existing OllamaClient instance
    client = OllamaClient(host="http://localhost:11434")
    instrumented_client = instrument_ollama_client(client)

    # All subsequent chat/chat_complete/generate calls are traced automatically.

Environment variables (set before importing):
    DD_LLMOBS_ENABLED=1
    DD_LLMOBS_ML_APP=monty-chatbot
    DD_LLMOBS_AGENTLESS_ENABLED=0
    DD_SERVICE=monty-llm
    DD_ENV=production
    DD_AGENT_HOST=192.168.50.179
"""

import functools
import json
import logging
import time

from ddtrace.llmobs import LLMObs

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Initialization
# ---------------------------------------------------------------------------

def enable_llm_monitoring(
    ml_app: str = "monty-chatbot",
    service_name: str = "monty-llm",
    agentless: bool = False,
):
    """
    Enable Datadog LLM Observability.

    This should be called once at application startup, before any LLM calls
    are made. It activates the LLMObs SDK which will automatically emit
    spans and metrics for instrumented LLM operations.

    Args:
        ml_app: The ML application name shown in Datadog LLM Observability.
        service_name: The APM service name for trace grouping.
        agentless: If True, send data directly to Datadog (requires API key).
                   If False (default), route through the local DD agent.
    """
    LLMObs.enable(
        ml_app=ml_app,
        integrations_enabled=True,
        agentless_enabled=agentless,
        service=service_name,
    )
    logger.info(
        "Datadog LLM Observability enabled: ml_app=%s, service=%s, agentless=%s",
        ml_app,
        service_name,
        agentless,
    )


def disable_llm_monitoring():
    """Disable Datadog LLM Observability and flush pending data."""
    LLMObs.disable()
    logger.info("Datadog LLM Observability disabled.")


# ---------------------------------------------------------------------------
# Span helpers
# ---------------------------------------------------------------------------

def _format_input_messages(messages) -> list:
    """Format messages into the LLMObs input_data structure."""
    formatted = []
    for msg in messages:
        role = getattr(msg, "role", "unknown")
        content = getattr(msg, "content", "")
        formatted.append({"role": role, "content": content})
    return formatted


def _format_output_message(response) -> list:
    """Format a ChatResponse into the LLMObs output_data structure."""
    message = getattr(response, "message", None)
    if message is None:
        return [{"role": "assistant", "content": ""}]
    content = getattr(message, "content", "")
    return [{"role": "assistant", "content": content}]


# ---------------------------------------------------------------------------
# Instrumented wrappers
# ---------------------------------------------------------------------------

def _wrap_chat(original_method):
    """
    Wrap OllamaClient.chat (streaming) with LLM Observability tracing.

    Because the streaming method yields chunks, we wrap the entire async
    generator and capture the final (done=True) chunk for token metrics.
    """

    @functools.wraps(original_method)
    async def wrapper(
        self,
        model,
        messages,
        tools=None,
        stream=True,
        think=None,
        options=None,
        keep_alive=None,
    ):
        span = LLMObs.llm(
            model_name=model,
            model_provider="ollama",
            name="ollama.chat_stream",
        )
        start_time = time.monotonic()

        LLMObs.annotate(
            span=span,
            input_data=_format_input_messages(messages),
            metadata={
                "model": model,
                "streaming": True,
                "tools_count": len(tools) if tools else 0,
                "think_enabled": bool(think),
            },
        )

        accumulated_content = []
        final_response = None
        completed = False

        try:
            async for chunk in original_method(
                self,
                model,
                messages,
                tools=tools,
                stream=stream,
                think=think,
                options=options,
                keep_alive=keep_alive,
            ):
                if hasattr(chunk, "message") and hasattr(chunk.message, "content"):
                    if chunk.message.content:
                        accumulated_content.append(chunk.message.content)

                if getattr(chunk, "done", False):
                    final_response = chunk

                yield chunk

            completed = True

        except Exception as exc:
            LLMObs.annotate(
                span=span,
                metadata={"error": str(exc), "error_type": type(exc).__name__},
            )
            raise

        finally:
            if completed or accumulated_content:
                output_text = "".join(accumulated_content)
                prompt_tokens = 0
                completion_tokens = 0

                if final_response is not None:
                    prompt_tokens = getattr(final_response, "prompt_eval_count", None) or 0
                    completion_tokens = getattr(final_response, "eval_count", None) or 0

                LLMObs.annotate(
                    span=span,
                    output_data=[{"role": "assistant", "content": output_text}],
                    metrics={
                        "input_tokens": prompt_tokens,
                        "output_tokens": completion_tokens,
                        "total_tokens": prompt_tokens + completion_tokens,
                    },
                )

                elapsed = time.monotonic() - start_time
                logger.debug(
                    "LLM stream traced: model=%s, prompt_tokens=%d, completion_tokens=%d, latency=%.3fs",
                    model,
                    prompt_tokens,
                    completion_tokens,
                    elapsed,
                )

            span.finish()

    return wrapper


def _wrap_generate(original_method):
    """
    Wrap OllamaClient.generate (streaming) with basic tracing.
    """

    @functools.wraps(original_method)
    async def wrapper(self, model, prompt, stream=True, images=None, options=None):
        span = LLMObs.llm(
            model_name=model,
            model_provider="ollama",
            name="ollama.generate",
        )
        start_time = time.monotonic()

        LLMObs.annotate(
            span=span,
            input_data=[{"role": "user", "content": prompt}],
            metadata={"model": model, "streaming": stream},
        )

        accumulated_response = []
        final_response = None

        try:
            async for chunk in original_method(
                self, model, prompt, stream=stream, images=images, options=options
            ):
                resp_text = getattr(chunk, "response", "")
                if resp_text:
                    accumulated_response.append(resp_text)

                if getattr(chunk, "done", False):
                    final_response = chunk

                yield chunk

            output_text = "".join(accumulated_response)
            prompt_tokens = 0
            completion_tokens = 0

            if final_response is not None:
                prompt_tokens = getattr(final_response, "prompt_eval_count", None) or 0
                completion_tokens = getattr(final_response, "eval_count", None) or 0

            LLMObs.annotate(
                span=span,
                output_data=[{"role": "assistant", "content": output_text}],
                metrics={
                    "input_tokens": prompt_tokens,
                    "output_tokens": completion_tokens,
                    "total_tokens": prompt_tokens + completion_tokens,
                },
            )

            elapsed = time.monotonic() - start_time
            logger.debug(
                "LLM generate traced: model=%s, latency=%.3fs",
                model,
                elapsed,
            )

        except Exception as exc:
            LLMObs.annotate(
                span=span,
                metadata={"error": str(exc), "error_type": type(exc).__name__},
            )
            raise

        finally:
            span.finish()

    return wrapper


def _wrap_embed(original_method):
    """
    Wrap OllamaClient.embed with an embedding span.
    """

    @functools.wraps(original_method)
    async def wrapper(self, model, input, options=None):
        span = LLMObs.embedding(
            model_name=model,
            model_provider="ollama",
            name="ollama.embed",
        )
        start_time = time.monotonic()

        input_text = input if isinstance(input, str) else " | ".join(input)
        LLMObs.annotate(
            span=span,
            input_data=input_text,
            metadata={"model": model},
        )

        try:
            result = await original_method(self, model, input, options=options)

            embedding_count = len(result.get("embeddings", []))
            LLMObs.annotate(
                span=span,
                output_data=f"Generated {embedding_count} embedding(s)",
                metadata={"embedding_count": embedding_count},
            )

            elapsed = time.monotonic() - start_time
            logger.debug(
                "Embedding traced: model=%s, count=%d, latency=%.3fs",
                model,
                embedding_count,
                elapsed,
            )

            return result

        except Exception as exc:
            LLMObs.annotate(
                span=span,
                metadata={"error": str(exc), "error_type": type(exc).__name__},
            )
            raise

        finally:
            span.finish()

    return wrapper


# ---------------------------------------------------------------------------
# Public API: Instrument an OllamaClient instance
# ---------------------------------------------------------------------------

def instrument_ollama_client(client):
    """
    Patch an OllamaClient instance so all LLM calls are traced via
    Datadog LLM Observability.

    This monkey-patches the following methods:
        - chat          (streaming)
        - generate      (streaming)
        - embed         (embeddings)

    Note: chat_complete is intentionally NOT wrapped because it internally
    calls self.chat(), which is already wrapped. Wrapping both would create
    duplicate spans.

    Args:
        client: An OllamaClient instance from agent.ollama.

    Returns:
        The same client instance, now instrumented.
    """
    import types

    # Bind the wrapped methods to the client instance
    client.chat = types.MethodType(
        _wrap_chat(client.chat.__func__
                   if hasattr(client.chat, "__func__")
                   else client.chat),
        client,
    )

    client.generate = types.MethodType(
        _wrap_generate(client.generate.__func__
                       if hasattr(client.generate, "__func__")
                       else client.generate),
        client,
    )

    client.embed = types.MethodType(
        _wrap_embed(client.embed.__func__
                    if hasattr(client.embed, "__func__")
                    else client.embed),
        client,
    )

    logger.info("OllamaClient instrumented with Datadog LLM Observability tracing.")
    return client


# ---------------------------------------------------------------------------
# Convenience: Decorator for arbitrary LLM workflow functions
# ---------------------------------------------------------------------------

def traced_llm_workflow(name: str = "llm.workflow"):
    """
    Decorator to create an LLMObs workflow span around a function.

    Usage:
        @traced_llm_workflow("agent.process_message")
        async def process_message(user_input: str) -> str:
            ...
    """

    def decorator(func):
        @functools.wraps(func)
        async def async_wrapper(*args, **kwargs):
            span = LLMObs.workflow(name=name)
            try:
                result = await func(*args, **kwargs)
                return result
            except Exception as exc:
                LLMObs.annotate(
                    span=span,
                    metadata={"error": str(exc), "error_type": type(exc).__name__},
                )
                raise
            finally:
                span.finish()

        @functools.wraps(func)
        def sync_wrapper(*args, **kwargs):
            span = LLMObs.workflow(name=name)
            try:
                result = func(*args, **kwargs)
                return result
            except Exception as exc:
                LLMObs.annotate(
                    span=span,
                    metadata={"error": str(exc), "error_type": type(exc).__name__},
                )
                raise
            finally:
                span.finish()

        import asyncio
        if asyncio.iscoroutinefunction(func):
            return async_wrapper
        return sync_wrapper

    return decorator
