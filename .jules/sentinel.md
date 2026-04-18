## 2025-02-28 - Insecure YAML Deserialization
**Vulnerability:** The codebase was using `yaml.load(f, Loader=yaml.FullLoader)` which can execute arbitrary code if the YAML file is malicious.
**Learning:** `yaml.load` is unsafe even with `FullLoader`. While `SafeLoader` is an option, `yaml.safe_load(f)` is the recommended safe replacement that only parses basic data types.
**Prevention:** Use `yaml.safe_load(f)` or `yaml.load(f, Loader=yaml.SafeLoader)` universally for loading YAML files unless custom object deserialization is strictly required and strictly controlled.
