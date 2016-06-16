function dump(x, name, indent) {
  var ret = name + "[" + typeof x + "] ";
  if (indent === undefined) indent = "";
  if (typeof x === 'object') {
    for (var prop in x) {
      ret += ("\n" + dump(x[prop], name + "." + prop, indent + "   "));
    }
    return indent + ret;
  }
  return indent + ret + "= " + x;
}

var res = $http.get('http://httpbin.org/get', {'a': 1, 'b': 2});
var jsonob = res.json();
print(dump(jsonob, "jsonob"));

