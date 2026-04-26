## 2025-02-23 - [Insecure YAML Deserialization]
**Vulnerability:** Found pervasive use of `yaml.load(f, Loader=yaml.FullLoader)` across multiple `kis_auth.py` files (in `examples_user/`, `examples_llm/`, and `legacy/`).
**Learning:** Even though `FullLoader` restricts execution compared to the default, it is still unsafe for parsing configuration files like `token.tmp` which should only contain simple data structures and timestamps. The code relied on `FullLoader` instead of the industry-standard `safe_load` when parsing API configuration and tokens.
**Prevention:** Always default to `yaml.safe_load()` for reading standard configuration files (like `.yaml` config and token caching files). Avoid `FullLoader` or `UnsafeLoader` unless specifically required and trusted.
