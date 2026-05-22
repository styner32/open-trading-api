## 2024-05-22 - Hide Sensitive Credentials in Legacy WebSocket Scripts
**Vulnerability:** Legacy WebSocket sample scripts in `legacy/websocket/python/` print `approval_key`, `aes_key`, and `aes_iv` to the console.
**Learning:** This exposes sensitive connection credentials to logs or stdout, which could be exploited if log files are accessible.
**Prevention:** Sensitive credential information should never be printed in plain text; either comment out the print statements or explicitly mask the data.
