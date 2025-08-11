import http from "k6/http";
import exec from 'k6/execution';
import { sleep } from "k6";
import { expect } from "https://jslib.k6.io/k6-testing/0.5.0/index.js";

const BASE_URL = __ENV.BASE_URL || 'https://quickpizza.grafana.com';

export const options = {
  stages: [
    { duration: "10s", target: 5 },
    { duration: "20s", target: 10 },
    { duration: "1s", target: 0 },
  ], 
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
  },{{ if .ProjectID }}
  cloud: {
    projectID: {{ .ProjectID }},
    name: "{{ .ScriptName }}",
  },{{ end }}
};

export function setup() {
  let res = http.get(BASE_URL);
  expect(res.status, `Got unexpected status code ${res.status} when trying to setup. Exiting.`).toBe(200);
}

export default function() {
  let restrictions = {
    maxCaloriesPerSlice: 500,
    mustBeVegetarian: false,
    excludedIngredients: ["pepperoni"],
    excludedTools: ["knife"],
    maxNumberOfToppings: 6,
    minNumberOfToppings: 2
  };

  let res = http.post(BASE_URL + "/api/pizza", JSON.stringify(restrictions), {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'token abcdef0123456789',
    },
  });

  expect.soft(res.status).toBe(200);
  console.log(res.json().pizza.name + " (" + res.json().pizza.ingredients.length + " ingredients)");
  sleep(1);
}
