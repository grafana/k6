import http from 'k6/http';

export const options = {
	iterations: 1,
    duration: __ENV.DURATION,
};

export default function () {
  const res = http.get('https://test.k6.io');
}
