## 2024-05-30 - Insecure YAML Deserialization
**Vulnerability:** Found `yaml.load` being used with `Loader=yaml.FullLoader` across multiple files to load `kis_devlp.yaml` configuration and `token.tmp` cached tokens. `yaml.load` even with `FullLoader` can be unsafe if parsing untrusted input.
**Learning:** Legacy and duplicated authentication scripts commonly use insecure `yaml.load`. In some cases it's used to parse tokens which contain timestamps, so `yaml.safe_load` must be verified to correctly handle the specific YAML inputs without breaking parsing.
**Prevention:** Always use `yaml.safe_load` for loading configuration and token cache files. Implement automated SAST checks (like Bandit) to block the introduction of `yaml.load`.
