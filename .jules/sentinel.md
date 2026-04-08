## 2024-04-08 - Fix Insecure YAML Deserialization
**Vulnerability:** Insecure deserialization using `yaml.load` (even with `Loader=yaml.FullLoader`).
**Learning:** `yaml.load` was used across multiple authentication and configuration scripts for parsing simple config and token data. `FullLoader` resolves all tags except those known to be unsafe, but `safe_load` is the strictly secure standard for data structures.
**Prevention:** Always use `yaml.safe_load()` for loading YAML data to prevent instantiation of arbitrary objects and potential code execution.
