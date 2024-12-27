import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || 'https://quickpizza.grafana.com';

export const options = {
  vus: 5,
  duration: '10s',{{ if .EnableCloud }}
  cloud: { {{ if .ProjectID }}
    projectID: {{ .ProjectID }}, {{ else }}
    // projectID: 12345, // Replace this with your own projectID {{ end }}
    name: "{{ .ScriptName }}",
  }, {{ end }}
};

export default function () {
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
      'X-User-ID': 23423,
    },
  });

  check(res, { "status is 200": (res) => res.status === 200 });
  console.log(res.json().pizza.name + " (" + res.json().pizza.ingredients.length + " ingredients)");
  sleep(1);
}
