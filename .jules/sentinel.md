## 2025-02-14 - [Insecure YAML Deserialization]
**Vulnerability:** Found widespread use of `yaml.load(..., Loader=yaml.FullLoader)` across multiple `kis_auth.py` and API scripts, which is susceptible to arbitrary code execution if an attacker controls the YAML file content (e.g., via a compromised configuration file or token temp file).
**Learning:** `yaml.load` even with `Loader=yaml.FullLoader` is unsafe against malicious payloads that can instantiate arbitrary Python objects. The codebase relied on this pattern for reading configuration files and token files (`kis_devlp.yaml`, `token.tmp`).
**Prevention:** Always use `yaml.safe_load()` when parsing YAML files from untrusted or potentially modifiable sources.
