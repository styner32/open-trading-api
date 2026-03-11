## 2024-11-09 - [CRITICAL] Fix arbitrary code execution vulnerability via unsafe YAML loading
**Vulnerability:** Codebase extensively uses `yaml.load(f, Loader=yaml.FullLoader)` to parse configuration files, which can execute arbitrary code inside a YAML file.
**Learning:** The initial implementation might not have been aware of the code execution vulnerability `yaml.load` exposes.
**Prevention:** Instead of `yaml.load`, only `yaml.safe_load` should be used which disables external code execution.
