
## 2024-05-24 - Exposed Credentials in Debug Logs
**Vulnerability:** Hardcoded `_DEBUG = True` flag caused request headers containing `appkey`, `appsecret`, and `Bearer tokens` to be logged to the console.
**Learning:** Hardcoding debug flags in production API wrappers leads to sensitive credential exposure during operations.
**Prevention:** Use environment variables (e.g., `KIS_DEBUG`) to control debug output and default to False.
