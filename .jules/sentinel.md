## 2024-05-18 - Prevent Arbitrary Code Execution via Unsafe YAML Loading
**Vulnerability:** Use of yaml.load() with yaml.FullLoader across multiple scripts can allow arbitrary code execution if an attacker provides a malicious YAML file.
**Learning:** Even with Loader=yaml.FullLoader, loading untrusted YAML data is dangerous. A full loader allows loading Python objects which can execute arbitrary code during instantiation.
**Prevention:** Always use yaml.safe_load() when parsing YAML files from potentially untrusted sources or configuration files to ensure only basic Python objects are created.
