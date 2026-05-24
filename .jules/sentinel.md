## 2024-05-24 - [CRITICAL] Prevent Sensitive Credentials Exposure in Logs
**Vulnerability:** Approval keys, AES keys, and AES IVs were printed directly to stdout via `print()` statements in legacy websocket sample files (e.g., `ws_domestic_stock.py`, `ws_domestic_future.py`, `multi_processing_sample_ws.py`, etc.).
**Learning:** Legacy debug logging patterns frequently output raw sensitive variables to stdout without considering the risk of exposing them in build logs or standard output captures.
**Prevention:** Always explicitly comment out or properly redact sensitive data (like `approval_key`, `aes_key`, `aes_iv`) in legacy sample code. Establish secure logging practices that mask sensitive variables by default.
