## YYYY-MM-DD - [Title]\n**Vulnerability:** [What you found]\n**Learning:** [Why it existed]\n**Prevention:** [How to avoid next time]

## 2025-02-18 - [Insecure YAML Loading]
**Vulnerability:** Found multiple instances of `yaml.load(f, Loader=yaml.FullLoader)` being used. `FullLoader` resolves all tags except those known to be unsafe, but `SafeLoader` (`yaml.safe_load`) is standard for just parsing standard YAML configurations. `yaml.load` can be vulnerable to arbitrary code execution if parsing untrusted data, and while `FullLoader` is safer than no loader, `SafeLoader` is widely recognized as best practice unless complex python objects need to be initialized.
**Learning:** `yaml.load(f, Loader=yaml.FullLoader)` was commonly used historically, but `yaml.safe_load(f)` is safer and widely required by security scanners to prevent execution of arbitrary functions.
**Prevention:** Always use `yaml.safe_load(f)` instead of `yaml.load(f, Loader=...)` for general configuration parsing.
