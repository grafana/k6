var http = require('http');
var res = http.get('http://httpbin.org/get', {'a': 1, 'b': 2});
print("URL: " + res.json().url);
