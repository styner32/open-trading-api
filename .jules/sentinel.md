## 2024-05-17 - Insecure YAML Deserialization
**Vulnerability:** Use of `yaml.load` even with `Loader=yaml.FullLoader` across the codebase which is vulnerable to insecure deserialization.
**Learning:** It existed because `yaml.load` is the traditional way to parse YAML, and `FullLoader` doesn't fully protect against arbitrary code execution when loading untrusted data.
**Prevention:** Always use `yaml.safe_load()` when parsing YAML files to avoid executing arbitrary code.
