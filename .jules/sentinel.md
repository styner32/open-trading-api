## 2024-04-21 - [YAML Deserialization]
**Vulnerability:** Insecure yaml deserialization using yaml.load()
**Learning:** Legacy codebase used unsafe default loaders which are susceptible to RCE
**Prevention:** Always use yaml.safe_load() when loading untrusted yaml files
