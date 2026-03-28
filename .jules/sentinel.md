## 2025-02-15 - [CRITICAL] Insecure YAML Loading Vulnerability
**Vulnerability:** Found multiple instances of `yaml.load(f, Loader=yaml.FullLoader)` being used to parse configuration files (`kis_devlp.yaml` and `config.yaml`). This is a known Remote Code Execution (RCE) risk because `yaml.FullLoader` can deserialize arbitrary Python objects.
**Learning:** `yaml.load()` was likely used due to historical examples or legacy documentation prioritizing functionality over security. Even with `Loader=yaml.FullLoader`, it remains unsafe against malicious YAML input.
**Prevention:** Always use `yaml.safe_load(f)` when parsing YAML files unless there is an explicit, well-documented need to instantiate custom Python objects. Security scanners should enforce the use of safe loaders.
