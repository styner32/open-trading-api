## 2024-05-24 - [Insecure YAML Deserialization]
**Vulnerability:** Found `yaml.load(f, Loader=yaml.FullLoader)` being used to parse configuration and token files, which can lead to arbitrary code execution (insecure deserialization).
**Learning:** The legacy codebase used `yaml.load` which was historically the default but is unsafe. Even with `FullLoader`, it's not completely secure against malicious payloads compared to `SafeLoader`.
**Prevention:** Always use `yaml.safe_load()` for parsing untrusted or external YAML files to ensure only basic Python objects are constructed.
