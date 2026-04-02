## 2024-05-18 - [Fix Insecure YAML Deserialization]
**Vulnerability:** The `yaml.load()` function was used with `Loader=yaml.FullLoader` to parse YAML configurations, which allows for arbitrary object construction and potentially arbitrary code execution if parsing untrusted data.
**Learning:** Even though configuration files (`kis_devlp.yaml`) are locally stored, any file parsed using PyYAML should use `yaml.safe_load()` as standard practice to mitigate risks if a file is tampered with by an external actor or process.
**Prevention:** Always use `yaml.safe_load()` for YAML parsing. The unsafe `yaml.load()` should never be used, even with `yaml.FullLoader`.
