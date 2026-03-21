## 2026-03-21 - [Insecure YAML Deserialization Pattern]
**Vulnerability:** Multiple instances of `yaml.load(..., Loader=yaml.FullLoader)` were used across `kis_auth.py` files and examples, which allows arbitrary code execution via unsafe YAML deserialization.
**Learning:** The legacy codebase and examples frequently copy-pasted an outdated snippet for parsing configuration (`kis_devlp.yaml`), repeating the unsafe pattern across different API scripts.
**Prevention:** Standardize on `yaml.safe_load()` for all YAML parsing to prevent arbitrary object instantiation. Any new example scripts must use the safe variant.
