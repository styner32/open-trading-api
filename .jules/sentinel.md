## 2024-05-24 - Credentials logging in websocket samples
**Vulnerability:** Legacy Python websocket sample scripts printed sensitive credentials (`approval_key`, `aes_key`, `aes_iv`) to standard output.
**Learning:** Example scripts often include debugging print statements that are insecure for production use or if users copy-paste the code as a starting point.
**Prevention:** Avoid logging credentials even in example code. Mask them or completely remove logging of such fields.
