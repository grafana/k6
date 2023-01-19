import http from 'k6/http';

export let options = {
  discardResponseBodies: true,
  scenarios: {
    Scenario_GetCrocodiles: {
      exec: 'FunctionForThisScenario',
      executor: 'ramping-vus',
      startTime: '0s',
      startVUs: 1,
      stages: [
        { duration: '5s', target: 10 },
      ],
    },
    Scenario_GetContacts: {
      exec: 'FunctionGetContacts',
      executor: 'ramping-vus',
      startTime: '3s',
      startVUs: 5,
      stages: [
        { duration: '2s', target: 5 },
      ],
    },    
  },
};

export function FunctionForThisScenario() {
  http.get('https://test-api.k6.io/public/crocodiles/');
}

export function FunctionGetContacts() {
  http.get('https://test.k6.io/contacts.php');
}