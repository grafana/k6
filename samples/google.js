var http = require('http');
var res = http.get("http://google.com/");
print(res.status);
sleep(1);
