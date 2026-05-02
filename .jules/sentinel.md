## 2025-02-18 - [Insecure YAML Deserialization]
**Vulnerability:** `yaml.load()` was used in multiple configuration parsing scripts (e.g., `kis_auth.py`, `kis_api.py`), which allows arbitrary Python code execution if a malicious YAML file is loaded.
**Learning:** Legacy scripts frequently use `yaml.load(..., Loader=yaml.FullLoader)` out of habit or older documentation. `yaml.FullLoader` is still unsafe for untrusted input.
**Prevention:** Always use `yaml.safe_load()` for parsing YAML configuration files in Python to prevent insecure deserialization.
