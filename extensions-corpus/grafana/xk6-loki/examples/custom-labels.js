import { check, sleep } from 'k6';
import loki from 'k6/x/loki';

const BASE_URL = "http://localhost:3100";
const KB = 1024;
const MB = KB * KB;

export const options = {
  vus: 1,
  iterations: 100,
};

const labels = loki.Labels({
  "format": ["logfmt"], // must contain at least one of the supported log formats
  "os": ["linux"],
  "cluster": ["k3d", "minikube"],
  "namespace": ["loki-prod", "loki-dev"],
  "container": ["distributor", "ingester", "querier", "query-frontend", "query-scheduler", "index-gateway", "compactor"],
  "instance": ["localhost"], // overrides the `instance` label which is otherwise derived from the hostname and VU
});

const conf = new loki.Config(BASE_URL, 10000, 1.0, {}, labels);
const client = new loki.Client(conf);

export default () => {
  let res = client.pushParameterized(10, 1 * MB, 2 * MB);
  check(res,
    {
      'successful write': (res) => {
        let success = res.status == 204;
        if (!success) console.log("write", res.status, res.body);
        return success;
      },
    }
  );
  let resp = client.labelsQuery("1m").body;
  let labels = JSON.parse(resp).data;
  labels.forEach((label) => {
    res = client.labelValuesQuery(label, "1m")
    check(res,
      {
        'successful read': (res) => {
          let success = res.status == 200;
          if (!success) console.log("read", label, res.status, res.body);
          return success;
        },
      }
    );
  })
  sleep(1);
};
