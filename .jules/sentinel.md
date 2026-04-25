## 2026-04-25 - [Hide Secrets in WebSockets]
**Vulnerability:** Legacy WebSocket samples in `legacy/websocket/python/` print sensitive credentials like 'approval_key', 'aes_key', and 'aes_iv' to standard output.
**Learning:** These print statements were likely used for debugging during development but can inadvertently leak sensitive security material in logs or consoles, violating data protection practices.
**Prevention:** Mask sensitive outputs using tools like placeholders (e.g., `[***]`) or remove the print statements altogether, particularly in reference implementations where developers might copy the code directly.
