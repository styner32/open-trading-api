## 2024-05-24 - [CRITICAL] Insecure YAML Deserialization
**Vulnerability:** Arbitrary code execution vulnerability through use of `yaml.load()` across multiple files parsing configuration data.
**Learning:** Legacy configuration loading relied on `yaml.load()` with `Loader=yaml.FullLoader` which is known to be unsafe.
**Prevention:** Always use `yaml.safe_load()` when parsing YAML configuration files to prevent arbitrary code execution attacks.
