// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

import http from "k6/http";
import { sleep, group } from "k6";

export let options = {
  discardResponseBodies: true,
  scenarios: {
    camel: {
      executor: "ramping-vus",
      startVUs: 1,
      stages: [
        { duration: "1m", target: 2 },
        { duration: "3m", target: 5 },
        { duration: "2m", target: 2 },
        { duration: "3m", target: 5 },
        { duration: "2m", target: 3 },
        { duration: "1m", target: 1 },
      ],
      gracefulRampDown: "0s",
    },
    snake: {
      executor: "ramping-vus",
      startVUs: 1,
      stages: [
        { duration: "1m", target: 1 },
        { duration: "1m", target: 4 },
        { duration: "1m", target: 1 },
        { duration: "1m", target: 4 },
        { duration: "1m", target: 1 },
        { duration: "1m", target: 4 },
        { duration: "1m", target: 1 },
        { duration: "1m", target: 4 },
        { duration: "1m", target: 1 },
        { duration: "1m", target: 4 },
        { duration: "1m", target: 1 },
        { duration: "1m", target: 1 },
      ],
      gracefulRampDown: "0s",
    },
  },
  thresholds: {
    http_req_duration: ["p(90) < 400", "avg <= 300"],
    iteration_duration: ["avg < 10000"],
  },
};

export default function () {
  group("main", () => {
    http.get("https://test-api.k6.io");
  });

  sleep(0.2);

  group("list", () => {
    http.get("https://test-api.k6.io/public/crocodiles/");
  });

  sleep(0.2);

  group("crocodiles", () => {
    for (var i = 0; i < 5; i++) {
      http.get(http.url`https://test-api.k6.io/public/crocodiles/${i}/`);
      sleep(0.5);
    }
  });
}
