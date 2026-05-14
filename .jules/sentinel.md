## 2024-05-14 - Replace Insecure YAML Deserialization
**Vulnerability:** Insecure YAML deserialization using `yaml.load(f, Loader=yaml.FullLoader)` instead of `yaml.safe_load(f)`.
**Learning:** `yaml.load` even with `FullLoader` can still be vulnerable to arbitrary code execution if untrusted YAML files are parsed, which is a known vulnerability in PyYAML. `yaml.safe_load` should always be used.
**Prevention:** Use `yaml.safe_load` for all YAML parsing.
