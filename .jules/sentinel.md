## 2023-10-27 - [Fix YAML load vulnerability]
**Vulnerability:** Insecure deserialization using yaml.load(f, Loader=yaml.FullLoader)
**Learning:** The FullLoader is capable of arbitrary code execution for any Python object. Legacy examples often used FullLoader instead of safe_load.
**Prevention:** Replace all yaml.load calls with yaml.safe_load(f)
