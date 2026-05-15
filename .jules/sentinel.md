## 2024-05-24 - [Insecure YAML Deserialization]
**Vulnerability:** Use of yaml.load() which allows arbitrary code execution.
**Learning:** The yaml.load() function was used with yaml.FullLoader to parse configuration files and tokens. While FullLoader restricts some execution, yaml.safe_load() is the standard and safest way to parse YAML without any risk of insecure deserialization.
**Prevention:** Always use yaml.safe_load() when parsing untrusted or local YAML files in Python.
