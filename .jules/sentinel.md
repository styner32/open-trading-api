## 2024-05-01 - [Insecure YAML Deserialization]
**Vulnerability:** The project used `yaml.load(f, Loader=yaml.FullLoader)` across multiple `kis_auth.py` files (and related API files) to load configurations from `.yaml` files.
**Learning:** `yaml.load` even with `FullLoader` can be risky and doesn't prevent all types of insecure deserialization, while `yaml.safe_load` specifically restricts loading to simple Python objects, preventing the execution of arbitrary Python functions. The vulnerability existed likely because older examples used `yaml.load` and were duplicated.
**Prevention:** Always use `yaml.safe_load(f)` or `yaml.SafeLoader` when deserializing YAML files, especially in globally-used utilities or configuration modules.
