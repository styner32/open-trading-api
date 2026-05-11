## 2025-02-14 - Fix yaml.load insecure deserialization and hardcoded debug leakage
**Vulnerability:** Found `yaml.load()` usage instead of `yaml.safe_load()` in multiple python scripts, opening up insecure deserialization risks. Also found hardcoded `_DEBUG = True` in `kis_api.py` which prints sensitive headers.
**Learning:** `yaml.load(Loader=yaml.FullLoader)` is safer than the default but `yaml.safe_load()` is the standard secure way for basic yaml parsing. Hardcoded debug mode risks token/auth info leaks to stdout.
**Prevention:** Use `yaml.safe_load()` instead of `yaml.load()`. Rely on environment variables instead of hardcoded `_DEBUG = True` values to toggle verbose debugging, especially in API wrappers handling sensitive data.
