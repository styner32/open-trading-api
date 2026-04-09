## 2024-04-09 - Insecure YAML Deserialization
**Vulnerability:** `yaml.load()` was used with `Loader=yaml.FullLoader` across multiple files for configuration parsing, which allows arbitrary code execution if a malicious YAML file is processed.
**Learning:** This existed because `yaml.FullLoader` is often mistakenly thought of as safe, or developers blindly copied boilerplate code without considering the security implications of YAML parsing.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()`. If complex Python objects really need to be serialized, consider safer formats or strictly validated custom loaders.
