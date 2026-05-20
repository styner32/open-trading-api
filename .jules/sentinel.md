## 2024-05-20 - [Hardcoded Debug Flag Exposing Secrets]
**Vulnerability:** Found `_DEBUG = True` hardcoded in `legacy/rest/kis_api.py`. When enabled, this flag causes sensitive HTTP headers (including authentication tokens and app keys) and payload bodies to be printed to the console in plain text (e.g., `print(f"<header>\n{headers}")`).
**Learning:** Hardcoded debug flags are often left behind from local testing and, when deployed, inadvertently expose sensitive credentials in logs or console output.
**Prevention:** Avoid hardcoding debug variables. Retrieve configuration settings from environment variables (e.g., `os.environ.get('KIS_DEBUG', 'False').lower() in ['true', '1', 't']`) and never log sensitive headers or payloads in production.
