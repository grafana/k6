import { check } from 'k6'
import http from 'k6/http'

export default function() {
  const responses = http.batch([
    "http://test.loadimpact.com",
    "http://test.loadimpact.com/pi.php",
    {
      "method": "GET",
      "url": "http://test.loadimpact.com/pi.php?decimals=50"
    }
  ]);

  check(responses[0], {
    "main page 200": res => res.status === 200,
  })

  check(responses[1], {
    "pi page 200": res => res.status === 200,
  })

  check(responses[2], {
    "pi page has 50 digits": res => {
      return res.body === "3.14159265358979323846264338327950288"
    }
  })
};
