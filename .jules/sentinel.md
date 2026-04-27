## 2024-05-24 - Insecure YAML Deserialization
**Vulnerability:** Found `yaml.load(f, Loader=yaml.FullLoader)` being used to parse configuration files.
**Learning:** `yaml.load` even with `FullLoader` can be exploited to execute arbitrary code via insecure deserialization. It should never be used on untrusted data. In this repository, the pattern was repeated across multiple auth files.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` when parsing YAML to prevent code execution.
