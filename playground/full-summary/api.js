import http from 'k6/http'
import {check, group} from 'k6'

export function apiTest() {
	const res = http.get('https://httpbin.org/get')
	check(res, {
		'test api is up': (r) => r.status === 200,
		'test api is 500': (r) => r.status === 500,
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
	})

	group('my crocodiles', () => {
		const res = http.get('https://httpbin.org/get')

		check(res, {
			'status is 200 OK': (r) => r.status === 200,
		})
	})
}