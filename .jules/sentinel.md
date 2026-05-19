## 2024-05-24 - [Fix insecure YAML loading]
**Vulnerability:** Insecure deserialization via `yaml.load()` in multiple python files
**Learning:** `yaml.load()` can execute arbitrary python functions if an attacker controls the yaml file. `Loader=yaml.FullLoader` is not safe against malicious input.
**Prevention:** Use `yaml.safe_load()` for reading config files
