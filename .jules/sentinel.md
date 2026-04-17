## 2024-05-15 - [Insecure YAML Deserialization]
**Vulnerability:** Found multiple instances of `yaml.load(f, Loader=yaml.FullLoader)` which is vulnerable to insecure deserialization if parsing untrusted input.
**Learning:** This existed likely because `FullLoader` was previously considered safe or was copied from outdated tutorials. Even `FullLoader` can be exploited in some scenarios, and `safe_load` is the modern standard for safely parsing YAML containing data.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` for parsing configuration or data files to prevent arbitrary code execution vulnerabilities.
