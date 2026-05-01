
## 2024-05-24 - Insecure YAML Deserialization
**Vulnerability:** Use of `yaml.load(f, Loader=yaml.FullLoader)` allows arbitrary code execution if a YAML file is manipulated.
**Learning:** The codebase used PyYAML's FullLoader extensively for loading configuration files and tokens across many authentication modules, indicating a pattern of unsafe default usage for local files.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` regardless of the loader specified, as it prevents construction of arbitrary Python objects.
