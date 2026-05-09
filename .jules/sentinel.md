## 2024-05-09 - Insecure YAML Deserialization
**Vulnerability:** Found multiple uses of `yaml.load` combined with `Loader=yaml.FullLoader` when parsing configuration and token files in `kis_auth.py` and other files.
**Learning:** This approach enables execution of arbitrary code (like `!!python/object/apply:os.system`) during parsing, leading to severe security risks (RCE). The previous implementations likely used `yaml.load` merely to deserialize a standard YAML file without understanding the full implications of using `FullLoader`.
**Prevention:** Consistently use `yaml.safe_load()` instead of `yaml.load()` when parsing YAML from untrusted or external files. `safe_load` correctly processes standard primitives without evaluating arbitrary object serialization tags.
