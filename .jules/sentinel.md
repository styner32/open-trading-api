## 2024-05-23 - [Critical] Insecure YAML Deserialization
**Vulnerability:** Found uses of `yaml.load` across multiple config parsing scripts which can lead to arbitrary code execution upon deserialization of maliciously crafted YAML data.
**Learning:** Legacy and quick-scripting approaches often favor `yaml.load` without considering its severe security implications.
**Prevention:** Always use `yaml.safe_load` for parsing YAML files.
