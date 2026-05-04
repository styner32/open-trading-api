## 2024-05-04 - Insecure Deserialization via yaml.load
**Vulnerability:** Multiple files use `yaml.load(f, Loader=yaml.FullLoader)` which is vulnerable to arbitrary code execution if parsing an untrusted YAML file.
**Learning:** Using `yaml.load` even with `FullLoader` is considered insecure.
**Prevention:** Always use `yaml.safe_load(f)` when parsing YAML files.
