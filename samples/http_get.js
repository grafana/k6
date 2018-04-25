import http from 'k6/http';
import { Counter} from "k6/metrics";

export function setup() {
  open("asdf");
}
// export let CounterErrors = new Counter("Errors");

// export let options = {
// 	thresholds: {
//     "Errors": [{ threshold: "count<5", abortOnFail: true }]
//   }
// };

export default function() {
  const response = http.get("http://test.loadimpact.com");
  // CounterErrors.add(true);
};
