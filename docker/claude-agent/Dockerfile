# Claude Agent Sidecar
# Node.js 22 + Python 3.12 + Claude Code CLI + dd_lib

FROM node:22-alpine

WORKDIR /app

# Install Python 3.12 and dependencies
RUN apk add --no-cache \
    python3 \
    py3-pip \
    py3-requests \
    git \
    curl \
    bash

# Copy dd_lib Python codebase
COPY dd_lib/ /app/dd_lib/

# Install Python dependencies (requests already installed via apk)
RUN pip3 install --break-system-packages requests || true

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Create working directories (use existing node user from base image)
RUN mkdir -p /app/work /app/assets && \
    chown -R node:node /app

# Copy agent server
COPY docker/claude-agent/agent-server.js /app/

# Set ownership
RUN chown -R node:node /app

USER node

# Expose HTTP API port for communication with main container
EXPOSE 9000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:9000/health || exit 1

# Set Python path for dd_lib
ENV PYTHONPATH=/app/dd_lib

CMD ["node", "/app/agent-server.js"]
