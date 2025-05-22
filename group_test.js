import { check, group } from 'k6';
import http from 'k6/http';

export default function () {
	group('Elmos Group', function () {
		// // define URL and payload
		// const url = 'https://quickpizza.grafana.com/api/users/token/login';
		// const payload = JSON.stringify({
		// 	username: 'default',
		// 	password: '12345678',
		// });
		//
		// const params = {
		// 	headers: {
		// 		'Content-Type': 'application/json',
		// 	},
		// };
		//
		// // send a post request and save response as a variable
		// const res = http.post(url, payload, params);
		// check(res, {
		// 	'is status 200': (r) => r.status === 200,
		// });
	});
	group('Irvines trashcan', function () {

	});
	group('Cookie monster', function () {
		// // define URL and payload
		// const url = 'https://quickpizza.grafana.com/api/users/token/login';
		// const payload = JSON.stringify({
		// 	username: 'default',
		// 	password: '12345678',
		// });
		//
		// const params = {
		// 	headers: {
		// 		'Content-Type': 'application/json',
		// 	},
		// };
		//
		// // send a post request and save response as a variable
		// const res = http.post(url, payload, params);
		// check(res, {
		// 	'is status 200': (r) => r.status === 200,
		// });
		group('Big Bird', function () {
			// // define URL and payload
			// const url = 'https://quickpizza.grafana.com/api/users/token/login';
			// const payload = JSON.stringify({
			// 	username: 'default',
			// 	password: '12345678',
			// });
			//
			// const params = {
			// 	headers: {
			// 		'Content-Type': 'application/json',
			// 	},
			// };
			//
			// // send a post request and save response as a variable
			// const res = http.post(url, payload, params);
			// check(res, {
			// 	'is status 200': (r) => r.status === 200,
			// });
			group('Abby Cadabby', function () {
				group('too much nesting', function () {
					// ...
				});
			});
		});

	});

}
