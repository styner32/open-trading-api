## 2024-05-15 - [Exposed Credentials in Debug Logs]
**Vulnerability:** Debug logging `print` statements in legacy scripts (`legacy/rest/kis_api.py`, `legacy/Sample01/kis_auth.py`) log entire HTTP headers, exposing highly sensitive credentials such as `authorization` (Bearer token), `appkey`, and `appsecret` in plaintext.
**Learning:** Legacy debug logic often dumps raw HTTP request and response objects for convenience, failing to filter or redact sensitive information that should never be logged or printed.
**Prevention:** Always implement explicit redaction or masking logic for sensitive keys (e.g., `authorization`, `appkey`, `appsecret`) before dumping header dictionaries or API responses to logs or console output.
