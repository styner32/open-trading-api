## 2025-02-13 - [Sentinel] yaml.load() Vulnerability
**Vulnerability:** Use of yaml.load() with Loader=yaml.FullLoader allows arbitrary code execution.
**Learning:** Even with FullLoader, yaml.load() can be unsafe. YAML parsing should always use yaml.safe_load() to prevent arbitrary code execution vulnerabilities unless full capabilities are explicitly needed.
**Prevention:** Always use yaml.safe_load() or yaml.load(f, Loader=yaml.SafeLoader) when parsing untrusted YAML data.
