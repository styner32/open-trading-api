## 2024-05-24 - [YAML Insecure Deserialization]
**Vulnerability:** Found `yaml.load(f, Loader=yaml.FullLoader)` being used across multiple authentication and API modules (e.g., `kis_auth.py`, `kis_api.py`).
**Learning:** `yaml.FullLoader` is safer than `yaml.Loader` but still allows object instantiation and is not recommended for untrusted input. `yaml.safe_load()` is the standard and properly parses the `token.tmp` files without risk.
**Prevention:** Always use `yaml.safe_load()` when parsing YAML configuration or token files to prevent insecure deserialization vulnerabilities.

## 2024-05-24 - [Credentials Logged in Output]
**Vulnerability:** Multiple legacy websocket scripts print sensitive credentials such as `approval_key`, `aes_key`, and `aes_iv` to standard output.
**Learning:** Print statements in example or development scripts that output authentication keys can easily leak credentials if the logs are captured or monitored by third parties.
**Prevention:** Ensure print statements containing sensitive keys are either removed, commented out, or explicitly masked before committing to the repository.
