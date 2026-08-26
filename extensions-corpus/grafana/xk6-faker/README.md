[![API Reference](https://img.shields.io/badge/API-reference-blue?logo=readme&logoColor=lightgray)](https://faker.x.k6.io)
[![GitHub Release](https://img.shields.io/github/v/release/grafana/xk6-faker)](https://github.com/grafana/xk6-faker/releases/)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/xk6-faker)](https://goreportcard.com/report/github.com/grafana/xk6-faker)
[![GitHub Actions](https://github.com/grafana/xk6-faker/actions/workflows/validate.yml/badge.svg)](https://github.com/grafana/xk6-faker/actions/workflows/validate.yml)
![GitHub Downloads](https://img.shields.io/github/downloads/grafana/xk6-faker/total)

# xk6-faker

**Random fake data generator for k6.**

Although there are several good JavaScript fake data generators, using these in [k6](https://k6.io) tests has several disadvantages (download size, memory usage, startup time, etc). The xk6-faker implemented as a golang extension, so tests start faster and use less memory. The price is a little bit smaller feature set compared with popular JavaScript fake data generators.

For convenience, the xk6-faker API resembles the popular [Faker.js](https://fakerjs.dev/). The category names and the generator function names are often different (due to the [underlying go faker library](https://github.com/brianvoe/gofakeit)), but the usage pattern is similar.

Check out the API documentation [here](https://faker.x.k6.io). The TypeScript declaration file can be downloaded from [here](https://faker.x.k6.io/index.d.ts).

To use the TypeScript declaration file in your IDE (e.g. Visual Studio Code), you need to create a `jsconfig.json` (or `tsconfig.json`) file with the following content:

```json file=examples/jsconfig.json
{
  "compilerOptions": {
    "target": "ES6",
    "module": "ES6",
    "paths": {
      "k6/x/faker": ["./typings/xk6-faker/index.d.ts"]
    }
  }
}
```

You will need to update the TypeScript declaration file location in the example above to where you downloaded it.


## Usage

For convenient use, the default export of the module is a Faker instance, it just needs to be imported and it is ready for use.

```ts file=examples/default-faker.js
import faker from "k6/x/faker";

export default function () {
  console.log(faker.person.firstName());
}

// prints a random first name
```

For a reproducible test run, a random seed value can be passed to the constructor of the Faker class.


```ts file=examples/custom-faker.js
import { Faker } from "k6/x/faker";

const faker = new Faker(11);

export default function () {
  console.log(faker.person.firstName());
}

// output: Josiah
```

Test reproducibility can also be achieved using the default Faker instance, if the seed value is set in the `XK6_FAKER_SEED` environment variable.

```bash
k6 run --env XK6_FAKER_SEED=11 script.js
```

then

```ts file=examples/default-faker-env.js
import faker from "k6/x/faker";

export default function () {
  console.log(faker.person.firstName());
}

// as long as XK6_FAKER_SEED is 11
// output: Josiah
```

The [examples](https://github.com/grafana/xk6-faker/blob/master/examples) directory contains examples of how to use the xk6-faker extension. A k6 binary containing the xk6-faker extension is required to run the examples.

> [!IMPORTANT]
> If the search path also contains the k6 command, don't forget to specify which k6 you want to run (for example `./k6`).

## Download

You can download pre-built k6 binaries from the [Releases](https://github.com/grafana/xk6-faker/releases/) page.

**Build**

The [xk6](https://github.com/grafana/xk6) build tool can be used to build a k6 that will include xk6-faker extension:

```bash
$ xk6 build --with github.com/grafana/xk6-faker@latest
```

For more build options and how to use xk6, check out the [xk6 documentation](https://github.com/grafana/xk6).

## Contribute

If you want to contribute or help with the development of **xk6-faker**, start by reading [CONTRIBUTING.md](CONTRIBUTING.md). 

## Feedback

If you find the xk6-faker extension useful, please star the repo. The number of stars will determine the time allocated for maintenance.

[![Stargazers over time](https://starchart.cc/grafana/xk6-faker.svg?variant=adaptive)](https://starchart.cc/grafana/xk6-faker)
