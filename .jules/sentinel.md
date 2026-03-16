## 2024-11-06 - Insecure YAML Deserialization
**Vulnerability:** The codebase was using `yaml.load(f, Loader=yaml.FullLoader)` which is vulnerable to arbitrary code execution if an attacker provides a maliciously crafted YAML file.
**Learning:** `yaml.load` is unsafe even with `FullLoader`, as it can instantiate arbitrary Python objects. This pattern was prevalent in multiple authentication and legacy REST scripts.
**Prevention:** Always use `yaml.safe_load(f)` or `yaml.load(f, Loader=yaml.SafeLoader)` when parsing untrusted YAML data to prevent insecure deserialization vulnerabilities.
