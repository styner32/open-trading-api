## 2024-06-25 - Print Statements Leaking Credentials
**Vulnerability:** Found multiple legacy Python websocket sample codes (`legacy/websocket/python/ws_*.py`) that printed `approval_key`, `aes_key`, and `aes_iv` directly to the standard output.
**Learning:** These print statements were likely meant for development or debugging, but were left in the codebase, potentially leaking sensitive authentication info to logs or console output.
**Prevention:** Mask or remove raw credential printing. Ensure that logs do not emit unredacted secrets or keys.
