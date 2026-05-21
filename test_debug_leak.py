import sys
import unittest
from unittest.mock import patch, mock_open, MagicMock

# Mock sys.modules for pandas since it might not be available
sys.modules['pandas'] = MagicMock()

import os
os.environ['KIS_DEBUG'] = 'true'

mock_yaml_data = """
my_agent: test_agent
my_url: http://test
my_app: test_app
my_sec: test_sec
"""

with patch('builtins.open', mock_open(read_data=mock_yaml_data)):
    import legacy.rest.kis_api as kis_api

    kis_api.getTREnv = MagicMock()
    kis_api.getTREnv().my_url = "http://test"
    kis_api.isPaperTrading = MagicMock(return_value=False)
    kis_api._getBaseHeader = MagicMock(return_value={
        "authorization": "secret_token",
        "appkey": "secret_appkey",
        "appsecret": "secret_appsecret",
        "other_header": "safe_value"
    })

    class TestDebugLeak(unittest.TestCase):
        @patch('sys.stdout', new_callable=MagicMock)
        @patch('legacy.rest.kis_api.requests')
        def test_debug_redaction(self, mock_requests, mock_stdout):
            # Setup mock response
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            mock_resp.headers = {'authorization': 'secret_token_resp', 'appkey': 'secret_appkey_resp', 'appsecret': 'secret_appsecret_resp', 'other': 'safe'}
            mock_resp.json.return_value = {'rt_cd': '0', 'msg1': 'OK', 'output1': []}
            mock_requests.get.return_value = mock_resp

            kis_api._url_fetch('/test', 'T123', {'param': 'val'}, postFlag=False)

            output = "\n".join([call[0][0] for call in mock_stdout.write.call_args_list])

            self.assertNotIn("secret_token", output)
            self.assertNotIn("secret_appkey", output)
            self.assertNotIn("secret_appsecret", output)
            self.assertNotIn("secret_token_resp", output)
            self.assertNotIn("secret_appkey_resp", output)
            self.assertNotIn("secret_appsecret_resp", output)

            self.assertIn("safe_value", output)
            self.assertIn("safe", output)

if __name__ == '__main__':
    unittest.main()
