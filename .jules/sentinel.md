## 2026-05-08 - Insecure YAML Deserialization
**Vulnerability:** Found uses of yaml.load() with FullLoader, which allows arbitrary code execution if a YAML file is tampered with.
**Learning:** Configuration parsing previously relied on unsafe methods that deserialized arbitrary Python objects.
**Prevention:** Always use yaml.safe_load() when parsing YAML files to ensure secure deserialization.
