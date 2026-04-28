## 2026-04-28 - [Insecure YAML Deserialization]
**Vulnerability:** The codebase was using `yaml.load(f, Loader=yaml.FullLoader)` which is vulnerable to insecure deserialization attacks if arbitrary yaml files can be processed.
**Learning:** Legacy configuration loading routines frequently utilized `yaml.load` which was common practice but is now considered unsafe in Python's PyYAML library.
**Prevention:** Always use `yaml.safe_load(f)` when deserializing YAML data. Use linting tools like Bandit or check codebase explicitly for `yaml.load`.
