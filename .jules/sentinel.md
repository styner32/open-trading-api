## 2026-04-24 - [Fix Insecure Deserialization in yaml.load]
**Vulnerability:** The use of `yaml.load(f, Loader=yaml.FullLoader)` allows arbitrary code execution via insecure deserialization of untrusted YAML input. It was found in multiple configuration and auth scripts across the codebase (e.g., kis_auth.py, get_ovsstk_chart_price.py).
**Learning:** `yaml.load` even with `FullLoader` is not fully secure and is a common pitfall. The legacy code inherited this vulnerable pattern for configuration parsing.
**Prevention:** Always use `yaml.safe_load()` for parsing YAML files to prevent arbitrary object instantiation and secure configuration loading.
