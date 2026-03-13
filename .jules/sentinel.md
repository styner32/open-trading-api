
## 2024-05-23 - [Insecure YAML Deserialization]
**Vulnerability:** Found multiple uses of `yaml.load(f, Loader=yaml.FullLoader)` to parse configuration files throughout the codebase. While `yaml.FullLoader` is safer than the default `Loader` in older PyYAML versions, it still allows the instantiation of arbitrary Python objects if the YAML contains specific tags.
**Learning:** `yaml.load()` can execute arbitrary code when parsing untrusted data, leading to a critical Remote Code Execution (RCE) vulnerability. `yaml.safe_load()` is the correct and standard safe way to parse YAML.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` when parsing YAML files to avoid arbitrary code execution vulnerabilities.
