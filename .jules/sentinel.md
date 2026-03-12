## 2024-05-20 - Insecure YAML Parsing
**Vulnerability:** Use of `yaml.load(f, Loader=yaml.FullLoader)` instead of `yaml.safe_load(f)`
**Learning:** `yaml.load` can execute arbitrary Python code from untrusted YAML files. `Loader=yaml.FullLoader` is safer than the default but still allows custom tag parsing which is not fully safe.
**Prevention:** Always use `yaml.safe_load(f)` when reading YAML files.
