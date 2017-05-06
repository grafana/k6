import http from 'k6/http';

export default function() {
  const response = http.get("http://test.loadimpact.com");
};
