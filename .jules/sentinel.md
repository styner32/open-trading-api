
## 2024-05-20 - [CRITICAL] Insecure YAML Deserialization in Config Parsing
**Vulnerability:** Several utility and example scripts (`kis_auth.py`, `kis_api.py`, etc.) were using `yaml.load(f, Loader=yaml.FullLoader)` to parse configuration files (`kis_devlp.yaml`) and cached token files (`token.tmp`). This is susceptible to arbitrary code execution if an attacker can manipulate the input YAML files.
**Learning:** `yaml.load` even with `Loader=yaml.FullLoader` is unsafe when loading untrusted data. The configuration and cached token files are often stored in common paths (`$HOME/KIS/config/` or local directory), making them a potential target for malicious manipulation.
**Prevention:** Always use `yaml.safe_load(f)` instead of `yaml.load(f, Loader=yaml.FullLoader)` when reading YAML configuration files and similar structured data to prevent insecure deserialization.
