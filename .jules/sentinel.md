## 2026-03-17 - Insecure YAML Parsing
**Vulnerability:** Use of yaml.load with FullLoader for configuration files
**Learning:** The FullLoader can instantiate arbitrary Python objects, posing a critical RCE risk when parsing untrusted YAML.
**Prevention:** Always use yaml.safe_load() to parse YAML files safely.
