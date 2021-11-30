import http from 'k6/http';
import { check, fail } from 'k6';
import { SharedArray } from 'k6/data';
import {
	SegmentedIndex,
	SharedSegmentedIndex,
} from 'k6/segment';

let data = new SharedArray('myarr', generateArray);

const params = {
		headers: { 'Content-type': 'application/json' },
};

export default function () {
	let iterator = new SharedSegmentedIndex('myarr'); // maybe data.name ?
	let index = iterator.next()

	const reqBody = JSON.stringify(data[index.unscaled])

	var res = http.post('https://httpbin.test.k6.io/anything', reqBody, params);
	check(res, { 'status 200': (r) => r.status === 200 });

	console.log(`Something: ${res.json().json.something}`)
}

function generateArray() {
	let n = 10;
	const arr = new Array(n);
	for (let i = 0; i < n; i++) {
		arr[i] = { something: 'something else ' + i, password: '12314561' };
	}
	return arr;
}
