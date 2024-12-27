import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '30s',{{ if .EnableCloud }}
  cloud: { {{ if .ProjectID }}
    projectID: {{ .ProjectID }}, {{ else }}
    // projectID: 12345, // Change this {{ end }}
    name: "{{ .ScriptName }}",
  }, {{ end }}
};

export default function () {
  http.get('https://quickpizza.grafana.com');
  sleep(1);
}
