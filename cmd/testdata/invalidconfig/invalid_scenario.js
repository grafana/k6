export const options = {
    scenarios: {
      example_scenario: {
        // name of the executor to use
        executor: 'shared-iterations',
        // common scenario configuration
        startTime: '10s',
        gracefulStop: '5s',
        env: { EXAMPLEVAR: 'testing' },
        tags: { example_tag: 'testing' },
  
        // executor-specific configuration
        vus: 10,
        iterations: 200,
        maxDuration: '10s',
      },
      another_scenario: {
        /*...*/
      },
    },
  };
  