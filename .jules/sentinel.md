## 2024-05-24 - [Information Leakage]
**Vulnerability:** In `legacy/rest/kis_api.py`, `_DEBUG = True` logs request headers (including `appkey`, `appsecret`, and `authorization` bearer token) to stdout via `print(f"<header>\n{headers}")` and `ar.printAll()`.
**Learning:** Hardcoded debug flags that print sensitive HTTP headers can leak API keys and bearer tokens to application logs, especially since the `_base_headers` dictionary is populated with these secrets.
**Prevention:** Use environment variables (e.g. `os.environ.get('KIS_DEBUG', 'False').lower() in ('true', '1', 't')`) to control debug logging, and default to `False`. Ensure that even in debug mode, sensitive headers like `authorization`, `appkey`, and `appsecret` are masked or redacted.
