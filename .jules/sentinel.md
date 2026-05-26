## 2024-05-26 - Sensitive Data Exposure in Legacy WebSocket Sample Codes
**Vulnerability:** Legacy Python WebSocket examples frequently exposed sensitive encryption keys (`aes_key`, `aes_iv`) and authentication credentials (`approval_key`, `APP_SECRET`, `ACCESS_TOKEN`) via `print()` statements used for debugging or demonstration.
**Learning:** This codebase's legacy samples were built with a focus on demonstrability rather than security, leading to a pattern of embedding direct outputs of sensitive cryptographic tokens and API secrets into standard output logs.
**Prevention:** Establish a strict policy to mask or comment out all debug prints involving authentication tokens, app secrets, and encryption keys in all example code and REST/WebSocket implementations.
