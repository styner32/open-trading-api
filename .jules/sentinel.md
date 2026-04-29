## 2024-05-24 - Insecure YAML Deserialization
**Vulnerability:** The codebase uses `yaml.load(f, Loader=yaml.FullLoader)` extensively across multiple legacy and example authentication and API scripts.
**Learning:** This pattern existed due to duplicating an insecure initial boilerplate for config and token loading. Even with `FullLoader`, `yaml.load` can execute arbitrary code upon deserialization, making it vulnerable to Remote Code Execution (RCE) if config files are maliciously altered.
**Prevention:** Always use `yaml.safe_load(f)` for loading YAML files. Avoid `yaml.load` unless explicitly required with a safe loader, but `safe_load` is the preferred secure alternative.
