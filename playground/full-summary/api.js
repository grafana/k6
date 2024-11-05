import http from 'k6/http'
import {check, group} from 'k6'

export function apiTest() {
	const res = http.get('https://httpbin.org/get')
	check(res, {
		'httpbin.org is up': (r) => r.status === 200,
		'httpbin.org is down': (r) => r.status === 500,
	})

	group('auth', () => {
		const res = http.post(
			'https://httpbin.org/auth',
			JSON.stringify({
				username: 'sakai',
				first_name: 'jin',
				last_name: 'sakai',
				email: 'jin.sakai@suckerpunch.com',
				password: 'onegaishimasu',
			})
		)
		check(res, {
			'status is 201 CREATED': (r) => r.status === 201,
		})

		group('authorized crocodiles', () => {
			const res = http.get('https://httpbin.org/get')

			check(res, {
				'authorized crocodiles are 200 OK': (r) => r.status === 200,
			})
		})
	})

	group('my crocodiles', () => {
		const res = http.get('https://httpbin.org/get')

		check(res, {
			'my crocodiles are 200 OK': (r) => r.status === 200,
		})
	})
}