## 2025-02-27 - Replace insecure yaml.load with yaml.safe_load
**Vulnerability:** The codebase was using `yaml.load(f, Loader=yaml.FullLoader)` to parse YAML files, which is vulnerable to arbitrary code execution (insecure deserialization) if a malicious YAML file is processed.
**Learning:** This likely existed because `yaml.load` is a common default or older practice before the widespread adoption of `yaml.safe_load` as the standard for parsing untrusted data safely.
**Prevention:** Always use `yaml.safe_load()` instead of `yaml.load()` when parsing YAML files unless there is a strict and validated requirement to instantiate custom Python objects from YAML, which should be avoided if possible.
