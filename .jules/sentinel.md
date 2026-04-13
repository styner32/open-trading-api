## 2025-02-12 - [Insecure YAML Deserialization]
**Vulnerability:** Found `yaml.load(f, Loader=yaml.FullLoader)` being used across multiple modules, including `kis_auth.py` and legacy REST scripts.
**Learning:** Even with `FullLoader`, `yaml.load` can be vulnerable to arbitrary code execution if untrusted YAML is parsed. `yaml.safe_load` should always be used for simple configurations.
**Prevention:** Use `yaml.safe_load()` for all YAML parsing tasks across the project to prevent insecure deserialization.
