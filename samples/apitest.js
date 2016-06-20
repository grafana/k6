// This script tries to exercise all API functions to make sure
// they exist, and work as intended.

// subsetof() checks if object A is a subset of object B
// using this primarily to check if we get back all the headers we sent to httpbin.org
function subsetof(a, b) {
  for (var prop in a) {
    if (!b.hasOwnProperty(prop))
      return false;
    if (typeof a[prop] !== typeof b[prop])
      return false;
    if (typeof a[prop] === 'object') {
      if (!subsetof(a[prop], b[prop]))
        return false;
    } else {
      if (JSON.stringify(a[prop]) !== JSON.stringify(b[prop]))
        return false;
    }
  }
  return true;
}

print("1. Testing log.debug()");
$log.debug("   log.debug() WORKS");
print("2. Testing log.info()");
$log.info("   log.info() WORKS");
print("3. Testing log.warn()");
$log.warn("   log.warn() WORKS");
print("4. Testing log.error()");
$log.error("   log.error() WORKS");

// test sleep() with float parameter
print("5. Testing vu.sleep(0.1)");
$vu.sleep(0.1);
// test sleep with int parameter
print("6. Testing vu.sleep(1)");
$vu.sleep(1);

print("7. Testing http.setMaxConnsPerHost()");
$http.setMaxConnsPerHost(4);
print("   http.setMaxConnsPerHost() seemingly WORKS");

var data = { 'a':'1', 'b':'2' };
var headers = { 'X-Myheader' : 'Myheadervalue', 'X-Myheader2' : 'Myheadervalue2' };
var params = { 'headers' : headers, 'quiet' : false }

print("8. Testing http.request(\"GET\", \"http://httpbin.org/get\")");
var jsondata = $http.request("GET", "http://httpbin.org/get", data, params).json();
if (!subsetof(data, jsondata.args)) {
  $log.debug("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
}
if (!subsetof(headers, jsondata.headers)) {
  $log.debug("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("9. Testing http.get(\"http://httpbin.org/get\")");
var jsondata = $http.get("http://httpbin.org/get", data, params).json();
if (!subsetof(data, jsondata.args)) {
  $log.debug("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
}
if (!subsetof(headers, jsondata.headers)) {
  $log.debug("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("10. Testing http.request(\"POST\", \"http://httpbin.org/post\")");
var jsondata = $http.request("POST", "http://httpbin.org/post", data, params).json();
// XXX TODO: verify that post data is returned
//if (!subsetof(data, jsondata.form)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("11. Testing http.post(\"http://httpbin.org/post\")");
var jsondata = $http.post("http://httpbin.org/post", data, params).json();
//if (!subsetof(data, jsondata.form)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("12. Testing http.request(\"PUT\", \"http://httpbin.org/put\")");
var jsondata = $http.request("PUT", "http://httpbin.org/put", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("13. Testing http.put(\"http://httpbin.org/put\")");
var jsondata = $http.put("http://httpbin.org/put", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("14. Testing http.request(\"DELETE\", \"http://httpbin.org/delete\")");
var jsondata = $http.request("DELETE", "http://httpbin.org/delete", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("15. Testing http.delete(\"http://httpbin.org/delete\")");
var jsondata = $http.delete("http://httpbin.org/delete", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("16. Testing http.request(\"PATCH\", \"http://httpbin.org/patch\")");
var jsondata = $http.request("PATCH", "http://httpbin.org/patch", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

print("17. Testing http.patch(\"http://httpbin.org/patch\")");
var jsondata = $http.patch("http://httpbin.org/patch", data, params).json();
//if (!subsetof(data, jsondata.args)) {
//  print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
//}
if (!subsetof(headers, jsondata.headers)) {
  print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
}

// 
// TODO: These do not exist on httpbin, find some other way to test them
// 

// print("18. Testing http.request(\"OPTIONS\", \"http://httpbin.org/options\")");
// var jsondata = $http.request("OPTIONS", "http://httpbin.org/options", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("19. Testing http.options(\"http://httpbin.org/options\")");
// var jsondata = $http.options("http://httpbin.org/options", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("20. Testing http.request(\"HEAD\", \"http://httpbin.org/head\")");
// var jsondata = $http.request("HEAD", "http://httpbin.org/head", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("21. Testing http.head(\"http://httpbin.org/head\")");
// var jsondata = $http.head("http://httpbin.org/head", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("22. Testing http.request(\"CONNECT\", \"http://httpbin.org/connect\")");
// var jsondata = $http.request("CONNECT", "http://httpbin.org/connect", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("23. Testing http.connect(\"http://httpbin.org/connect\")");
// var jsondata = $http.connect("http://httpbin.org/connect", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("24. Testing http.request(\"TRACE\", \"http://httpbin.org/trace\")");
// var jsondata = $http.request("TRACE", "http://httpbin.org/trace", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

// print("25. Testing http.trace(\"http://httpbin.org/trace\")");
// var jsondata = $http.trace("http://httpbin.org/trace", data, params).json();
// if (!subsetof(data, jsondata.args)) {
//   print("ERROR!  I sent: " + JSON.stringify(data) + " but got back: " + JSON.stringify(jsondata.args))
// }
// if (!subsetof(headers, jsondata.headers)) {
//   print("ERROR!  I sent: " + JSON.stringify(headers) + " but got back: " + JSON.stringify(jsondata.headers))
// }

print("26. Testing vu.id()");
print("   vu.id() = " + $vu.id() + " -- IT WORKS");

print("27. Testing vu.iteration()");
print("   vu.iteration() = " + $vu.iteration() + " -- IT WORKS");

print("28. Testing test.url()");
print("   test.url() = " + $test.url() + " -- IT WORKS");

print("29. Testing test.abort()");
$test.abort();
