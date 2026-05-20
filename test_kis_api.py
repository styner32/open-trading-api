import os
os.environ["KIS_DEBUG"] = "False"
import legacy.rest.kis_api as kis_api

print("Debug is:", kis_api._DEBUG)
