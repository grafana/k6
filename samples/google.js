var http = require('http');
var res = http.get("http://google.com/", null, { report: true });
print(res.status);
sleep(1);
