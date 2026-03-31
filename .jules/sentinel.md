## 2025-02-13 - Unsafe YAML Deserialization
**Vulnerability:** Using `yaml.load(..., Loader=yaml.FullLoader)` can lead to arbitrary code execution if untrusted YAML is parsed. Found in multiple authentication and configuration parsing scripts.
**Learning:** `yaml.load` is unsafe even with `FullLoader`, as it permits executing Python code embedded in YAML.
**Prevention:** Always use `yaml.safe_load(...)` which limits loading to simple Python objects like dictionaries and lists, eliminating the risk of arbitrary code execution.
