// file contains overloads for the web platform test, to make it work with k6
// javascript runtime.


// this is a minimal implementation of the promise_test function
// which used in many web platform tests
function promise_test(fn, name) {
   try {
       fn();  
   } catch (e) {
       throw Error(`Error in test "${name}": ${e}`);
   }
}

// this is a minimal implementation of the done function
// which used in many web platform tests
function done() {}

// this is a minimal implementation of the setup function
function setup() {}

// some tests use the self object, so we need to define it
globalThis.self = globalThis;