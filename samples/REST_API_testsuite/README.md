REST API test suite
===================

This is a small test suite showing a possible way to write test cases for individual REST API
end points using k6. 

The end points tested here are actual REST API end points used by loadimpact.com (api.loadimpact.com).

There is one test case per API endpoint - each contained in one JS file. The test cases are runnable 
both as functional tests and as load tests. 


Files
-----

* common.js
  
  This should not be executed. It is only for including by the test case scripts, and contains global parameters used by all test cases.
  
* v3_account_login.js

  This tests the /v3/account/login end point, incidentally logging in a user and getting a security/API token back.
  
* v3_account_me.js

  Test for the /v3/account/me end point.

* v3_load-zones.js

  Test for the /v3/load-zones end point.

* v3_server-agents.js

  Test for the /v3/server-agents end point.

* v3_tokens.js

  Test for the /v3/tokens end point.

