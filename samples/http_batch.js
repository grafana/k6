import { check } from 'k6';
import http from 'k6/http';

export default function() {
  const responses = http.batch([
    "http://test.loadimpact.com",
    "http://test.loadimpact.com/pi.php"
  ]);

  check(responses[0], {
    "main page 200": res => res.status === 200,
  });

  check(responses[1], {
    "pi page 200": res => res.status === 200,
    "pi page has right content": res => res.body === "3.14",
  });
};
