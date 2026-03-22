## 2024-05-24 - Insecure YAML Deserialization
**Vulnerability:** Found `yaml.load(f, Loader=yaml.FullLoader)` being used to parse YAML configuration files across the codebase (`kis_auth.py`, legacy scripts).
**Learning:** `yaml.load` (even with `FullLoader`) can be tricked into instantiating arbitrary Python objects, leading to arbitrary code execution if a maliciously crafted YAML file is loaded.
**Prevention:** Always use `yaml.safe_load(f)` when parsing YAML files from untrusted sources or as a general security best practice, as it safely restricts object instantiation to basic Python types.
