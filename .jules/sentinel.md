## 2024-05-18 - Insecure YAML Deserialization
**Vulnerability:** Found multiple instances of `yaml.load(f, Loader=yaml.FullLoader)` being used to parse configuration and token files.
**Learning:** `yaml.load` is susceptible to insecure deserialization attacks even with `FullLoader`, leading to arbitrary code execution. The codebase has a widespread pattern of using it for configuration loading.
**Prevention:** Always use `yaml.safe_load(f)` when parsing YAML files from untrusted or dynamically generated sources.
