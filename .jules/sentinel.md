
## 2024-05-24 - Insecure YAML Deserialization
**Vulnerability:** Found uses of `yaml.load(f, Loader=yaml.FullLoader)` which can lead to arbitrary code execution if parsing untrusted YAML.
**Learning:** It existed because `yaml.load` is a legacy pattern, and even with `Loader=yaml.FullLoader`, it is not strictly safe for fully untrusted input compared to `yaml.safe_load()`.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` when parsing YAML files to prevent insecure deserialization vulnerabilities.
