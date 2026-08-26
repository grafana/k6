/**
 * **Random fake data generator for k6.**
 *
 * Altought there is several good JavaScript fake data generator,
 * but using these in [k6](https://k6.io) tests has several disadvantages (download size, memory usage, startup time, etc).
 * The xk6-faker implemented as a golang extension, so tests starts faster and use less memory.
 * The price is a little bit smaller feature set compared with popular JavaScript fake data generators.
 *
 * For convenience, the xk6-faker API resembles the popular [Faker.js](https://fakerjs.dev/).
 * The category names and the generator function names are often different
 * (due to the [underlying go faker library](https://github.com/brianvoe/gofakeit)),
 * but the way of use is similar.
 *
 * @example
 * For convenient use, the default export of the module is a Faker instance,
 * it just needs to be imported and it is ready for use.
 *
 * ```ts
 * import faker from "k6/x/faker"
 *
 * export default function() {
 *   console.log(faker.person.firstName())
 * }
 *
 * // prints a random first name
 * ```
 * For a reproducible test run, a random seed value can be passed to the constructor of the Faker class.
 *
 * ```ts
 * import { Faker } from "k6/x/faker"
 *
 * const faker = new Faker(11)
 *
 * export default function() {
 *   console.log(faker.person.firstName())
 * }
 * ```
 * **Output** (formatted as JSON value)
 * ```json
 * "Josiah"
 * ```
 * The reproducibility of the test can also be achieved using the default Faker instance,
 * if the seed value is set in the `XK6_FAKER_SEED` environment variable.
 * ```bash
 * k6 run --env XK6_FAKER_SEED=11 script.js
 * ```
 * then
 * ```ts
 * import faker from "k6/x/faker"
 *
 * export default function() {
 *   console.log(faker.person.firstName())
 * }
 * ```
 * **Output** (formatted as JSON value)
 * ```json
 * "Josiah"
 * ```
 *
 * @module faker
 */
export as namespace faker;

/**
 * This is the faker module's main class containing all generators that can be used to generate data.
 *
 * Please have a look at the individual generators and methods for more information.
 *
 * @example
 * ```ts
 * import { Faker } from "k6/x/faker"
 *
 * const faker = new Faker(11)
 *
 * export default function() {
 *   console.log(faker.person.firstName())
 * }
 * ```
 * **Output** (formatted as JSON value)
 * ```json
 * "Josiah"
 * ```
 */
export declare class Faker {
  /**
   * Creates a new instance of Faker.
   *
   * Optionally, the value of the random seed can be set as a constructor parameter.
   * This is intended to allow for consistent values in a tests,
   * so you might want to use hardcoded values as the seed.
   *
   * Please note that generated values are dependent on both the seed and the number
   * of calls that have been made.
   *
   * Setting seed to 0 (or omitting it) will use seed derived from system entropy.
   *
   * @param seed random seed value for deterministic generator
   *
   * @example
   * ```ts
   * const consistentFaker = new Faker(11)
   * const semiRandomFaker = new Faker()
   * ```
   */
  constructor(seed?: number);

  /**
   * Call fake data generator function based on function name.
   *
   * @param func the name of the generator function to be called
   * @param args parameters for the generator function to be called
   */
  call(func: string, ...args: unknown[]): unknown;
}
