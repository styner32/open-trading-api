## 2024-03-24 - [Insecure YAML Deserialization]
**Vulnerability:** Use of `yaml.load()` even with `Loader=yaml.FullLoader` allows arbitrary code execution during deserialization.
**Learning:** In python, `yaml.load()` with `FullLoader` is still dangerous and `yaml.safe_load()` should be the default for parsing YAML configurations or tokens in order to protect against malicious input execution while correctly parsing standard configurations.
**Prevention:** Always use `yaml.safe_load()` rather than `yaml.load()` throughout the codebase.
