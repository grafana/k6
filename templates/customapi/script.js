import http from 'k6/http';
import { check } from 'k6';

// Custom API Test Template
export const options = {
  vus: 5,
  duration: '1m',{{ if .ProjectID }}
  cloud: {
    projectID: {{ .ProjectID }},
    name: "{{ .ScriptName }}",
  },{{ end }}
};

export default function() {
  // This is a custom local template!
  let response = http.get('https://api.example.com/health');
  check(response, {
    'status is 200': (r) => r.status === 200,
  });
} 