
## 2024-06-25 - [Fix Unsafe YAML Parsing]
**Vulnerability:** The codebase was using `yaml.load(f, Loader=yaml.FullLoader)` which is vulnerable to arbitrary code execution if a malicious YAML payload is supplied.
**Learning:** Legacy YAML parsing snippets were copied across multiple files (`examples`, `legacy/rest`, `legacy/Sample01`), repeating the insecure `yaml.load()` pattern instead of `yaml.safe_load()`. This is a common pattern when example code is copy-pasted.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` for parsing YAML files to avoid arbitrary code execution, especially for configuration files.
