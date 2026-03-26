## 2026-03-26 - Prevent arbitrary code execution in YAML loading
**Vulnerability:** Use of yaml.load() with yaml.FullLoader across the codebase.
**Learning:** Using yaml.load() is dangerous even with FullLoader.
**Prevention:** Always use yaml.safe_load() when loading YAML files.
