export {apiTest} from './api.js';
export {browserTest} from './browser.js';
export {grpcTest} from './grpc.js';
export {wsTest} from './ws.js';

export const options = {
	thresholds: {
		'http_reqs{group: ::auth}': ['count>1', 'count<5'],
		'http_reqs{scenario: api}': ['count>1'],
	},
	scenarios: {
		api: {
			executor: 'per-vu-iterations',
			vus: 1,
			iterations: 1,
			exec: 'apiTest',
		},
		browser: {
			executor: 'shared-iterations',
			options: {
				browser: {
					type: 'chromium',
				},
			},
			exec: 'browserTest',
		},
		grpc: {
			executor: 'shared-iterations',
			exec: 'grpcTest',
		},
		ws: {
			executor: 'shared-iterations',
			exec: 'wsTest',
		},
	},
}
