// Monty Chat WebSocket Client (montyWorldWide - no tools for public safety)
//
// Protocol-aware WebSocket connection:
//   - HTTPS (via Cloudflare tunnel): use wss:// with the public gateway domain
//   - HTTP  (local dev):             use ws://  with the LAN IP
//
// TODO: Add a `gateway.n0kos.com` (or `api.n0kos.com`) subdomain route in the
//       Cloudflare tunnel config and DNS once the Monty gateway is ready to be
//       exposed publicly. The tunnel ingress rule should point to
//       http://192.168.50.68:8001 (same as the LAN target below).

const MONTY_LAN_HOST = '192.168.50.68';
const MONTY_LAN_PORT = '8001';
const MONTY_PUBLIC_GATEWAY = 'gateway.n0kos.com';

const isSecure = window.location.protocol === 'https:';
const MONTY_WS_URL = isSecure
    ? `wss://${MONTY_PUBLIC_GATEWAY}/chat/ws`
    : `ws://${MONTY_LAN_HOST}:${MONTY_LAN_PORT}/chat/ws`;

let ws = null;
let conversationId = null;
let isConnected = false;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 3000;

// DOM Elements
function getElements() {
    return {
        toggle: document.getElementById('chat-toggle'),
        container: document.getElementById('chat-container'),
        messages: document.getElementById('chat-messages'),
        input: document.getElementById('chat-input'),
        sendBtn: document.getElementById('chat-send'),
        status: document.getElementById('chat-status'),
        chatIcon: document.querySelector('.chat-icon'),
        closeIcon: document.querySelector('.close-icon'),
    };
}

// Toggle chat window
function toggleChat() {
    const { container, chatIcon, closeIcon } = getElements();
    const isOpen = !container.classList.contains('hidden');

    if (isOpen) {
        container.classList.add('hidden');
        chatIcon.classList.remove('hidden');
        closeIcon.classList.add('hidden');
    } else {
        container.classList.remove('hidden');
        chatIcon.classList.add('hidden');
        closeIcon.classList.remove('hidden');

        // Connect if not connected
        if (!isConnected) {
            connectWebSocket();
        }

        // Focus input
        setTimeout(() => {
            const { input } = getElements();
            input.focus();
        }, 100);
    }
}

// Update connection status
function updateStatus(status, isOnline = false) {
    const { status: statusEl } = getElements();
    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('.status-text');

    text.textContent = status;
    dot.classList.toggle('online', isOnline);
    dot.classList.toggle('offline', !isOnline);
}

// Connect to WebSocket
function connectWebSocket() {
    if (ws && ws.readyState === WebSocket.OPEN) {
        return;
    }

    updateStatus('Connecting...');

    try {
        ws = new WebSocket(MONTY_WS_URL);

        ws.onopen = () => {
            isConnected = true;
            reconnectAttempts = 0;
            updateStatus('Online', true);
            console.log('Connected to Monty');
        };

        ws.onclose = () => {
            isConnected = false;
            updateStatus('Disconnected');
            console.log('Disconnected from Monty');

            // Attempt reconnect
            if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                setTimeout(connectWebSocket, RECONNECT_DELAY);
            }
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            updateStatus('Connection error');
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                handleMessage(data);
            } catch (e) {
                console.error('Failed to parse message:', e);
            }
        };
    } catch (error) {
        console.error('Failed to connect:', error);
        updateStatus('Failed to connect');
    }
}

// Handle incoming messages
let currentMessageEl = null;
let currentContent = '';

function handleMessage(data) {
    const { messages } = getElements();

    switch (data.event_type || data.type) {
        case 'classification':
            // Agent was assigned - show thinking indicator
            if (!currentMessageEl) {
                currentMessageEl = createMessageElement('assistant');
                currentMessageEl.classList.add('thinking');
                currentMessageEl.querySelector('.message-content').innerHTML =
                    '<span class="thinking-dots"><span>.</span><span>.</span><span>.</span></span>';
                messages.appendChild(currentMessageEl);
                scrollToBottom();
            }
            break;

        case 'thinking':
            // Model is thinking
            if (currentMessageEl && data.thinking) {
                const thinkingEl = currentMessageEl.querySelector('.thinking-text');
                if (!thinkingEl) {
                    const contentEl = currentMessageEl.querySelector('.message-content');
                    contentEl.innerHTML = `<div class="thinking-text">${escapeHtml(data.thinking)}</div>`;
                } else {
                    thinkingEl.textContent += data.thinking;
                }
            }
            break;

        case 'content':
            // Actual response content
            if (!currentMessageEl) {
                currentMessageEl = createMessageElement('assistant');
                messages.appendChild(currentMessageEl);
            }

            currentMessageEl.classList.remove('thinking');
            currentContent += data.content || '';

            const contentEl = currentMessageEl.querySelector('.message-content');
            contentEl.innerHTML = formatMessage(currentContent);
            scrollToBottom();
            break;

        case 'tool_executing':
            // Show tool being executed
            if (currentMessageEl) {
                const toolInfo = document.createElement('div');
                toolInfo.className = 'tool-info';
                toolInfo.innerHTML = `<span class="tool-icon">&#9881;</span> Running ${escapeHtml(data.tool_name)}...`;
                currentMessageEl.querySelector('.message-content').appendChild(toolInfo);
                scrollToBottom();
            }
            break;

        case 'tool_result':
            // Tool completed
            break;

        case 'done':
        case 'completed':
            // Message complete
            currentMessageEl = null;
            currentContent = '';
            break;

        case 'error':
            addErrorMessage(data.error_message || data.error || 'An error occurred');
            currentMessageEl = null;
            currentContent = '';
            break;
    }

    // Store conversation ID
    if (data.conversation_id) {
        conversationId = data.conversation_id;
    }
}

// Create message element
function createMessageElement(role) {
    const div = document.createElement('div');
    div.className = `chat-message ${role}`;
    div.innerHTML = '<div class="message-content"></div>';
    return div;
}

// Add user message to UI
function addUserMessage(text) {
    const { messages } = getElements();
    const messageEl = createMessageElement('user');
    messageEl.querySelector('.message-content').textContent = text;
    messages.appendChild(messageEl);
    scrollToBottom();
}

// Add error message
function addErrorMessage(error) {
    const { messages } = getElements();
    // Remove thinking indicator if present
    removeThinkingIndicator();
    const messageEl = createMessageElement('error');
    messageEl.querySelector('.message-content').textContent = error;
    messages.appendChild(messageEl);
    scrollToBottom();
}

// Show thinking indicator
function showThinkingIndicator() {
    const { messages } = getElements();
    // Create thinking message if not exists
    if (!currentMessageEl) {
        currentMessageEl = createMessageElement('assistant');
        currentMessageEl.classList.add('thinking');
        currentMessageEl.id = 'thinking-indicator';
        currentMessageEl.querySelector('.message-content').innerHTML =
            '<span class="thinking-dots"><span>.</span><span>.</span><span>.</span></span>';
        messages.appendChild(currentMessageEl);
        scrollToBottom();
    }
}

// Remove thinking indicator
function removeThinkingIndicator() {
    const indicator = document.getElementById('thinking-indicator');
    if (indicator && currentMessageEl === indicator) {
        // Don't remove, will be replaced with actual content
    }
}

// Send message
function sendMessage() {
    const { input } = getElements();
    const message = input.value.trim();

    if (!message) return;

    if (!isConnected) {
        addErrorMessage('Not connected to Monty. Reconnecting...');
        connectWebSocket();
        return;
    }

    // Add user message to UI
    addUserMessage(message);

    // Show thinking indicator immediately
    showThinkingIndicator();

    // Send via WebSocket
    ws.send(JSON.stringify({
        message: message,
        conversation_id: conversationId,
    }));

    // Clear input
    input.value = '';
}

// Handle Enter key
function handleKeyDown(event) {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        sendMessage();
    }
}

// Scroll messages to bottom
function scrollToBottom() {
    const { messages } = getElements();
    messages.scrollTop = messages.scrollHeight;
}

// Format message with basic markdown
function formatMessage(text) {
    // Escape HTML first
    let formatted = escapeHtml(text);

    // Code blocks
    formatted = formatted.replace(/```(\w*)\n?([\s\S]*?)```/g, '<pre><code>$2</code></pre>');

    // Inline code
    formatted = formatted.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Bold
    formatted = formatted.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');

    // Italic
    formatted = formatted.replace(/\*([^*]+)\*/g, '<em>$1</em>');

    // Line breaks
    formatted = formatted.replace(/\n/g, '<br>');

    return formatted;
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    // Pre-connect for faster response when chat is opened
    // Uncomment below to auto-connect on page load:
    // connectWebSocket();
});
