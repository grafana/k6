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
 * @module k6/x/faker
 */
declare module "k6/x/faker" {

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
  export class Faker {
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


    /**
     * Generator to generate addresses and locations.
     */
    readonly address: Address;

    /**
     * Generator to generate animals.
     */
    readonly animal: Animal;

    /**
     * Generator to generate application related entries.
     */
    readonly app: App;

    /**
     * Generator to generate beer related entries.
     */
    readonly beer: Beer;

    /**
     * Generator to generate book related entries.
     */
    readonly book: Book;

    /**
     * Generator to generate car related entries.
     */
    readonly car: Car;

    /**
     * Generator to generate celebrities.
     */
    readonly celebrity: Celebrity;

    /**
     * Generator to generate colors.
     */
    readonly color: Color;

    /**
     * Generator to generate company related entries.
     */
    readonly company: Company;

    /**
     * Generator to generate emoji related entries.
     */
    readonly emoji: Emoji;

    /**
     * Generator to generate various error codes and messages.
     */
    readonly error: Error;

    /**
     * Generator to generate file related entries.
     */
    readonly file: File;

    /**
     * Generator to generate finance related entries.
     */
    readonly finance: Finance;

    /**
     * Generator to generate food related entries.
     */
    readonly food: Food;

    /**
     * Generator to generate game related entries.
     */
    readonly game: Game;

    /**
     * Generator to generate hacker/IT words and phrases.
     */
    readonly hacker: Hacker;

    /**
     * Generator to generate hipster words, phrases and paragraphs.
     */
    readonly hipster: Hipster;

    /**
     * Generator to generate internet related entries.
     */
    readonly internet: Internet;

    /**
     * Generator to generate language related entries.
     */
    readonly language: Language;

    /**
     * Generator to generate minecraft related entries.
     */
    readonly minecraft: Minecraft;

    /**
     * Generator to generate movie related entries.
     */
    readonly movie: Movie;

    /**
     * Generator to generate numbers.
     */
    readonly numbers: Numbers;

    /**
     * Generator to generate payment related entries.
     */
    readonly payment: Payment;

    /**
     * Generator to generate people's personal information.
     */
    readonly person: Person;

    /**
     * Generator to generate product related entries.
     */
    readonly product: Product;

    /**
     * Generator to generate strings.
     */
    readonly strings: Strings;

    /**
     * Generator to generate time and date.
     */
    readonly time: Time;

    /**
     * Generator to generate words and sentences.
     */
    readonly word: Word;

    /**
     * Generator with all generator functions for convenient use.
     */
    readonly zen: Zen;
  }

  /** Default Faker instance. */
  const faker: Faker;

  /** Default Faker instance */
  export default faker;

  /**
   * Generator to generate addresses and locations.
   */
  export interface Address {
    /**
     * Residential location including street, city, state, country and postal code.
     * @returns a random address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Address":"53883 Villageborough, San Bernardino, Kentucky 56992","Street":"53883 Villageborough","City":"San Bernardino","State":"Kentucky","Zip":"56992","Country":"United States of America","Latitude":11.29359,"Longitude":-145.577493}
     * ```
     */
    address(): Record<string, unknown>;

    /**
     * Part of a country with significant population, often a central hub for culture and commerce.
     * @returns a random city
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.city())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hialeah"
     * ```
     */
    city(): string;

    /**
     * Nation with its own government and defined territory.
     * @returns a random country
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.country())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Togo"
     * ```
     */
    country(): string;

    /**
     * Shortened 2-letter form of a country's name.
     * @returns a random country abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.countryAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "TG"
     * ```
     */
    countryAbbreviation(): string;

    /**
     * Geographic coordinate specifying north-south position on Earth's surface.
     * @returns a random latitude
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.latitude())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 11.394086
     * ```
     */
    latitude(): number;

    /**
     * Latitude number between the given range (default min=0, max=90).
     * @param min - Min
     * @param max - Max
     * @returns a random latitude range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.latitudeRange(0,90))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 50.697043
     * ```
     */
    latitudeRange(min: number, max: number): number;

    /**
     * Geographic coordinate indicating east-west position on Earth's surface.
     * @returns a random longitude
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.longitude())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 22.788172
     * ```
     */
    longitude(): number;

    /**
     * Longitude number between the given range (default min=0, max=180).
     * @param min - Min
     * @param max - Max
     * @returns a random longitude range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.longitudeRange(0,180))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 101.394086
     * ```
     */
    longitudeRange(min: number, max: number): number;

    /**
     * Governmental division within a country, often having its own laws and government.
     * @returns a random state
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.state())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Massachusetts"
     * ```
     */
    state(): string;

    /**
     * Shortened 2-letter form of a country's state.
     * @returns a random state abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.stateAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "AA"
     * ```
     */
    stateAbbreviation(): string;

    /**
     * Public road in a city or town, typically with houses and buildings on each side.
     * @returns a random street
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.street())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "53883 Villageborough"
     * ```
     */
    street(): string;

    /**
     * Name given to a specific road or street.
     * @returns a random street name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.streetName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Fall"
     * ```
     */
    streetName(): string;

    /**
     * Numerical identifier assigned to a street.
     * @returns a random street number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.streetNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25388"
     * ```
     */
    streetNumber(): string;

    /**
     * Directional or descriptive term preceding a street name, like 'East' or 'Main'.
     * @returns a random street prefix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.streetPrefix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "West"
     * ```
     */
    streetPrefix(): string;

    /**
     * Designation at the end of a street name indicating type, like 'Avenue' or 'Street'.
     * @returns a random street suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.streetSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ville"
     * ```
     */
    streetSuffix(): string;

    /**
     * Numerical code for postal address sorting, specific to a geographic area.
     * @returns a random zip
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.address.zip())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25388"
     * ```
     */
    zip(): string;
  }

  /**
   * Generator to generate animals.
   */
  export interface Animal {
    /**
     * Living creature with the ability to move, eat, and interact with its environment.
     * @returns a random animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.animal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "crow"
     * ```
     */
    animal(): string;

    /**
     * Type of animal, such as mammals, birds, reptiles, etc..
     * @returns a random animal type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.animalType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "amphibians"
     * ```
     */
    animalType(): string;

    /**
     * Distinct species of birds.
     * @returns a random bird
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.bird())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lovebird"
     * ```
     */
    bird(): string;

    /**
     * Various breeds that define different cats.
     * @returns a random cat
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.cat())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Toyger"
     * ```
     */
    cat(): string;

    /**
     * Various breeds that define different dogs.
     * @returns a random dog
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.dog())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Staffordshire Bullterrier"
     * ```
     */
    dog(): string;

    /**
     * Animal name commonly found on a farm.
     * @returns a random farm animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.farmAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Cow"
     * ```
     */
    farmAnimal(): string;

    /**
     * Affectionate nickname given to a pet.
     * @returns a random pet name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.animal.petName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Nacho"
     * ```
     */
    petName(): string;
  }

  /**
   * Generator to generate application related entries.
   */
  export interface App {
    /**
     * Person or group creating and developing an application.
     * @returns a random app author
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.app.appAuthor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Wendell Luettgen"
     * ```
     */
    appAuthor(): string;

    /**
     * Software program designed for a specific purpose or task on a computer or mobile device.
     * @returns a random app name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.app.appName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hillbe"
     * ```
     */
    appName(): string;

    /**
     * Particular release of an application in Semantic Versioning format.
     * @returns a random app version
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.app.appVersion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "5.3.20"
     * ```
     */
    appVersion(): string;
  }

  /**
   * Generator to generate beer related entries.
   */
  export interface Beer {
    /**
     * Measures the alcohol content in beer.
     * @returns a random beer alcohol
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerAlcohol())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "6.5%"
     * ```
     */
    beerAlcohol(): string;

    /**
     * Scale indicating the concentration of extract in worts.
     * @returns a random beer blg
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerBlg())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "13.4°Blg"
     * ```
     */
    beerBlg(): string;

    /**
     * The flower used in brewing to add flavor, aroma, and bitterness to beer.
     * @returns a random beer hop
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerHop())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Nugget"
     * ```
     */
    beerHop(): string;

    /**
     * Scale measuring bitterness of beer from hops.
     * @returns a random beer ibu
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerIbu())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "80 IBU"
     * ```
     */
    beerIbu(): string;

    /**
     * Processed barley or other grains, provides sugars for fermentation and flavor to beer.
     * @returns a random beer malt
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerMalt())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Roasted barley"
     * ```
     */
    beerMalt(): string;

    /**
     * Specific brand or variety of beer.
     * @returns a random beer name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "90 Minute IPA"
     * ```
     */
    beerName(): string;

    /**
     * Distinct characteristics and flavors of beer.
     * @returns a random beer style
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerStyle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "English Brown Ale"
     * ```
     */
    beerStyle(): string;

    /**
     * Microorganism used in brewing to ferment sugars, producing alcohol and carbonation in beer.
     * @returns a random beer yeast
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.beer.beerYeast())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2035 - American Lager"
     * ```
     */
    beerYeast(): string;
  }

  /**
   * Generator to generate book related entries.
   */
  export interface Book {
    /**
     * Written or printed work consisting of pages bound together, covering various subjects or stories.
     * @returns a random book
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.book.book())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Title":"The Brothers Karamazov","Author":"Albert Camus","Genre":"Urban"}
     * ```
     */
    book(): Record<string, string>;

    /**
     * The individual who wrote or created the content of a book.
     * @returns a random book author
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.book.bookAuthor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Edgar Allan Poe"
     * ```
     */
    bookAuthor(): string;

    /**
     * Category or type of book defined by its content, style, or form.
     * @returns a random book genre
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.book.bookGenre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Erotic"
     * ```
     */
    bookGenre(): string;

    /**
     * The specific name given to a book.
     * @returns a random book title
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.book.bookTitle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "The Brothers Karamazov"
     * ```
     */
    bookTitle(): string;
  }

  /**
   * Generator to generate car related entries.
   */
  export interface Car {
    /**
     * Wheeled motor vehicle used for transportation.
     * @returns a random car
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.car())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Type":"Passenger car compact","Fuel":"CNG","Transmission":"Automatic","Brand":"Daewoo","Model":"Thunderbird","Year":1905}
     * ```
     */
    car(): Record<string, unknown>;

    /**
     * Type of energy source a car uses.
     * @returns a random car fuel type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.carFuelType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ethanol"
     * ```
     */
    carFuelType(): string;

    /**
     * Company or brand that manufactures and designs cars.
     * @returns a random car maker
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.carMaker())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Lancia"
     * ```
     */
    carMaker(): string;

    /**
     * Specific design or version of a car produced by a manufacturer.
     * @returns a random car model
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.carModel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tucson 4wd"
     * ```
     */
    carModel(): string;

    /**
     * Mechanism a car uses to transmit power from the engine to the wheels.
     * @returns a random car transmission type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.carTransmissionType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Manual"
     * ```
     */
    carTransmissionType(): string;

    /**
     * Classification of cars based on size, use, or body style.
     * @returns a random car type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.car.carType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Passenger car compact"
     * ```
     */
    carType(): string;
  }

  /**
   * Generator to generate celebrities.
   */
  export interface Celebrity {
    /**
     * Famous person known for acting in films, television, or theater.
     * @returns a random celebrity actor
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.celebrity.celebrityActor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ben Affleck"
     * ```
     */
    celebrityActor(): string;

    /**
     * High-profile individual known for significant achievements in business or entrepreneurship.
     * @returns a random celebrity business
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.celebrity.celebrityBusiness())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Larry Ellison"
     * ```
     */
    celebrityBusiness(): string;

    /**
     * Famous athlete known for achievements in a particular sport.
     * @returns a random celebrity sport
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.celebrity.celebritySport())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Greg Lemond"
     * ```
     */
    celebritySport(): string;
  }

  /**
   * Generator to generate colors.
   */
  export interface Color {
    /**
     * Hue seen by the eye, returns the name of the color like red or blue.
     * @returns a random color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.color.color())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "MediumVioletRed"
     * ```
     */
    color(): string;

    /**
     * Six-digit code representing a color in the color model.
     * @returns a random hex color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.color.hexColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "#bd38ac"
     * ```
     */
    hexColor(): string;

    /**
     * Attractive and appealing combinations of colors, returns an list of color hex codes.
     * @returns a random nice colors
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.color.niceColors())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "MediumVioletRed"
     * ```
     */
    niceColors(): string[];

    /**
     * Color defined by red, green, and blue light values.
     * @returns a random rgb color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.color.rgbColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [13,150,143]
     * ```
     */
    rgbColor(): number[];

    /**
     * Colors displayed consistently on different web browsers and devices.
     * @returns a random safe color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.color.safeColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "black"
     * ```
     */
    safeColor(): string;
  }

  /**
   * Generator to generate company related entries.
   */
  export interface Company {
    /**
     * Brief description or summary of a company's purpose, products, or services.
     * @returns a random blurb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.blurb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Pride"
     * ```
     */
    blurb(): string;

    /**
     * Random bs company word.
     * @returns a random bs
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.bs())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "24-7"
     * ```
     */
    bs(): string;

    /**
     * Trendy or overused term often used in business to sound impressive.
     * @returns a random buzzword
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.buzzword())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Reverse-engineered"
     * ```
     */
    buzzword(): string;

    /**
     * Designated official name of a business or organization.
     * @returns a random company
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.company())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Xatori"
     * ```
     */
    company(): string;

    /**
     * Suffix at the end of a company name, indicating business structure, like 'Inc.' or 'LLC'.
     * @returns a random company suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.companySuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "LLC"
     * ```
     */
    companySuffix(): string;

    /**
     * Position or role in employment, involving specific tasks and responsibilities.
     * @returns a random job
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.job())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Company":"Xatori","Title":"Representative","Descriptor":"Future","Level":"Tactics"}
     * ```
     */
    job(): Record<string, string>;

    /**
     * Word used to describe the duties, requirements, and nature of a job.
     * @returns a random job descriptor
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.jobDescriptor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Internal"
     * ```
     */
    jobDescriptor(): string;

    /**
     * Random job level.
     * @returns a random job level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.jobLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Identity"
     * ```
     */
    jobLevel(): string;

    /**
     * Specific title for a position or role within a company or organization.
     * @returns a random job title
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.jobTitle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Representative"
     * ```
     */
    jobTitle(): string;

    /**
     * Catchphrase or motto used by a company to represent its brand or values.
     * @returns a random slogan
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.company.slogan())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Pride. De-engineered!"
     * ```
     */
    slogan(): string;
  }

  /**
   * Generator to generate emoji related entries.
   */
  export interface Emoji {
    /**
     * Digital symbol expressing feelings or ideas in text messages and online chats.
     * @returns a random emoji
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.emoji.emoji())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "🐮"
     * ```
     */
    emoji(): string;

    /**
     * Alternative name or keyword used to represent a specific emoji in text or code.
     * @returns a random emoji alias
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.emoji.emojiAlias())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "slovakia"
     * ```
     */
    emojiAlias(): string;

    /**
     * Group or classification of emojis based on their common theme or use, like 'smileys' or 'animals'.
     * @returns a random emoji category
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.emoji.emojiCategory())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Smileys & Emotion"
     * ```
     */
    emojiCategory(): string;

    /**
     * Brief explanation of the meaning or emotion conveyed by an emoji.
     * @returns a random emoji description
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.emoji.emojiDescription())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "disguised face"
     * ```
     */
    emojiDescription(): string;

    /**
     * Label or keyword associated with an emoji to categorize or search for it easily.
     * @returns a random emoji tag
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.emoji.emojiTag())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lick"
     * ```
     */
    emojiTag(): string;
  }

  /**
   * Generator to generate various error codes and messages.
   */
  export interface Error {
    /**
     * A problem or issue encountered while accessing or managing a database.
     * @returns a random database error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.databaseError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    databaseError(): string;

    /**
     * Message displayed by a computer or software when a problem or mistake is encountered.
     * @returns a random error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.error())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    error(): string;

    /**
     * Various categories conveying details about encountered errors.
     * @returns a random error object word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.errorObjectWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    errorObjectWord(): string;

    /**
     * Communication failure in the high-performance, open-source universal RPC framework.
     * @returns a random grpc error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.gRPCError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    gRPCError(): string;

    /**
     * Failure or issue occurring within a client software that sends requests to web servers.
     * @returns a random http client error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.httpClientError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpClientError(): string;

    /**
     * A problem with a web http request.
     * @returns a random http error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.httpError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpError(): string;

    /**
     * Failure or issue occurring within a server software that recieves requests from clients.
     * @returns a random http server error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.httpServerError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpServerError(): string;

    /**
     * Malfunction occuring during program execution, often causing abrupt termination or unexpected behavior.
     * @returns a random runtime error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.runtimeError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    runtimeError(): string;

    /**
     * Occurs when input data fails to meet required criteria or format specifications.
     * @returns a random validation error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.error.validationError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    validationError(): string;
  }

  /**
   * Generator to generate file related entries.
   */
  export interface File {
    /**
     * Suffix appended to a filename indicating its format or type.
     * @returns a random file extension
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.file.fileExtension())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "max"
     * ```
     */
    fileExtension(): string;

    /**
     * Defines file format and nature for browsers and email clients using standardized identifiers.
     * @returns a random file mime type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.file.fileMimeType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "text/html"
     * ```
     */
    fileMimeType(): string;
  }

  /**
   * Generator to generate finance related entries.
   */
  export interface Finance {
    /**
     * Unique identifier for securities, especially bonds, in the United States and Canada.
     * @returns a random cusip
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.finance.cusip())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "S4BL2MVY6"
     * ```
     */
    cusip(): string;

    /**
     * International standard code for uniquely identifying securities worldwide.
     * @returns a random isin
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.finance.isin())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "MYS4BL2MVY69"
     * ```
     */
    isin(): string;
  }

  /**
   * Generator to generate food related entries.
   */
  export interface Food {
    /**
     * First meal of the day, typically eaten in the morning.
     * @returns a random breakfast
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.breakfast())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ham omelet deluxe"
     * ```
     */
    breakfast(): string;

    /**
     * Sweet treat often enjoyed after a meal.
     * @returns a random dessert
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.dessert())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Lindas bloodshot eyeballs"
     * ```
     */
    dessert(): string;

    /**
     * Evening meal, typically the day's main and most substantial meal.
     * @returns a random dinner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.dinner())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Asian broccoli salad"
     * ```
     */
    dinner(): string;

    /**
     * Liquid consumed for hydration, pleasure, or nutritional benefits.
     * @returns a random drink
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.drink())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Water"
     * ```
     */
    drink(): string;

    /**
     * Edible plant part, typically sweet, enjoyed as a natural snack or dessert.
     * @returns a random fruit
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.fruit())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Avocado"
     * ```
     */
    fruit(): string;

    /**
     * Midday meal, often lighter than dinner, eaten around noon.
     * @returns a random lunch
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.lunch())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tortellini skewers"
     * ```
     */
    lunch(): string;

    /**
     * Random snack.
     * @returns a random snack
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.snack())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hoisin marinated wing pieces"
     * ```
     */
    snack(): string;

    /**
     * Edible plant or part of a plant, often used in savory cooking or salads.
     * @returns a random vegetable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.food.vegetable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Broccoli"
     * ```
     */
    vegetable(): string;
  }

  /**
   * Generator to generate game related entries.
   */
  export interface Game {
    /**
     * Small, cube-shaped objects used in games of chance for random outcomes.
     * @param numdice - Number of Dice
     * @param sides - Number of Sides
     * @returns a random dice
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.game.dice(1,[5,4,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [5]
     * ```
     */
    dice(numdice: number, sides: number[]): number[];

    /**
     * User-selected online username or alias used for identification in games.
     * @returns a random gamertag
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.game.gamertag())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "BraveArmadillo"
     * ```
     */
    gamertag(): string;
  }

  /**
   * Generator to generate hacker/IT words and phrases.
   */
  export interface Hacker {
    /**
     * Abbreviations and acronyms commonly used in the hacking and cybersecurity community.
     * @returns a random hacker abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackerAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "GB"
     * ```
     */
    hackerAbbreviation(): string;

    /**
     * Adjectives describing terms often associated with hackers and cybersecurity experts.
     * @returns a random hacker adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackerAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "auxiliary"
     * ```
     */
    hackerAdjective(): string;

    /**
     * Noun representing an element, tool, or concept within the realm of hacking and cybersecurity.
     * @returns a random hacker noun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackerNoun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "application"
     * ```
     */
    hackerNoun(): string;

    /**
     * Informal jargon and slang used in the hacking and cybersecurity community.
     * @returns a random hacker phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackerPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Try to transpile the EXE sensor, maybe it will deconstruct the wireless interface!"
     * ```
     */
    hackerPhrase(): string;

    /**
     * Verbs associated with actions and activities in the field of hacking and cybersecurity.
     * @returns a random hacker verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackerVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "read"
     * ```
     */
    hackerVerb(): string;

    /**
     * Verb describing actions and activities related to hacking, often involving computer systems and security.
     * @returns a random hackering verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hacker.hackeringVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quantifying"
     * ```
     */
    hackeringVerb(): string;
  }

  /**
   * Generator to generate hipster words, phrases and paragraphs.
   */
  export interface Hipster {
    /**
     * Paragraph showcasing the use of trendy and unconventional vocabulary associated with hipster culture.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random hipster paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hipster.hipsterParagraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Offal forage pinterest direct trade pug. Skateboard food truck flannel cold-pressed church-key.<br />Keffiyeh wolf pop-up jean shorts before they sold out. Hoodie roof portland intelligentsia gastropub."
     * ```
     */
    hipsterParagraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * Sentence showcasing the use of trendy and unconventional vocabulary associated with hipster culture.
     * @param wordcount - Word Count
     * @returns a random hipster sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hipster.hipsterSentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Offal forage pinterest direct trade pug."
     * ```
     */
    hipsterSentence(wordcount: number): string;

    /**
     * Trendy and unconventional vocabulary used by hipsters to express unique cultural preferences.
     * @returns a random hipster word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.hipster.hipsterWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "offal"
     * ```
     */
    hipsterWord(): string;
  }

  /**
   * Generator to generate internet related entries.
   */
  export interface Internet {
    /**
     * The specific identification string sent by the Google Chrome web browser when making requests on the internet.
     * @returns a random chrome user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.chromeUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (X11; Linux i686) AppleWebKit/5340 (KHTML, like Gecko) Chrome/40.0.816.0 Mobile Safari/5340"
     * ```
     */
    chromeUserAgent(): string;

    /**
     * Human-readable web address used to identify websites on the internet.
     * @returns a random domain name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.domainName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "internalenhance.org"
     * ```
     */
    domainName(): string;

    /**
     * The part of a domain name that comes after the last dot, indicating its type or purpose.
     * @returns a random domain suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.domainSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "info"
     * ```
     */
    domainSuffix(): string;

    /**
     * The specific identification string sent by the Firefox web browser when making requests on the internet.
     * @returns a random firefox user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.firefoxUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (Macintosh; U; PPC Mac OS X 10_9_1 rv:5.0) Gecko/1979-07-30 Firefox/37.0"
     * ```
     */
    firefoxUserAgent(): string;

    /**
     * Verb used in HTTP requests to specify the desired action to be performed on a resource.
     * @returns a random http method
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.httpMethod())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "HEAD"
     * ```
     */
    httpMethod(): string;

    /**
     * Random http status code.
     * @returns a random http status code
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.httpStatusCode())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 400
     * ```
     */
    httpStatusCode(): number;

    /**
     * Three-digit number returned by a web server to indicate the outcome of an HTTP request.
     * @returns a random http status code simple
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.httpStatusCodeSimple())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 200
     * ```
     */
    httpStatusCodeSimple(): number;

    /**
     * Number indicating the version of the HTTP protocol used for communication between a client and a server.
     * @returns a random http version
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.httpVersion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "HTTP/1.0"
     * ```
     */
    httpVersion(): string;

    /**
     * Web address pointing to an image file that can be accessed and displayed online.
     * @param width - Width
     * @param height - Height
     * @returns a random image url
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.imageUrl(500,500))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "https://picsum.photos/500/500"
     * ```
     */
    imageUrl(width: number, height: number): string;

    /**
     * Attribute used to define the name of an input element in web forms.
     * @returns a random input name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.inputName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "last_name"
     * ```
     */
    inputName(): string;

    /**
     * Numerical label assigned to devices on a network for identification and communication.
     * @returns a random ipv4 address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.ipv4Address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "234.106.177.171"
     * ```
     */
    ipv4Address(): string;

    /**
     * Numerical label assigned to devices on a network, providing a larger address space than IPv4 for internet communication.
     * @returns a random ipv6 address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.ipv6Address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "3aea:ef6a:38b1:7cab:7f0:946c:a3a9:cb90"
     * ```
     */
    ipv6Address(): string;

    /**
     * Classification used in logging to indicate the severity or priority of a log entry.
     * @returns a random log level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.logLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "error"
     * ```
     */
    logLevel(): string;

    /**
     * Unique identifier assigned to network interfaces, often used in Ethernet networks.
     * @returns a random mac address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.macAddress())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "87:2d:cd:bc:0d:f3"
     * ```
     */
    macAddress(): string;

    /**
     * The specific identification string sent by the Opera web browser when making requests on the internet.
     * @returns a random opera user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.operaUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Opera/10.45 (X11; Linux i686; en-US) Presto/2.13.288 Version/13.00"
     * ```
     */
    operaUserAgent(): string;

    /**
     * Secret word or phrase used to authenticate access to a system or account.
     * @param lower - Lower
     * @param upper - Upper
     * @param numeric - Numeric
     * @param special - Special
     * @param space - Space
     * @param length - Length
     * @returns a random password
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.password(true,false,true,true,false,12))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "z42x8h!47-9r"
     * ```
     */
    password(lower: boolean, upper: boolean, numeric: boolean, special: boolean, space: boolean, length: number): string;

    /**
     * The specific identification string sent by the Safari web browser when making requests on the internet.
     * @returns a random safari user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.safariUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (iPhone; CPU iPhone OS 7_3_2 like Mac OS X; en-US) AppleWebKit/534.34.8 (KHTML, like Gecko) Version/3.0.5 Mobile/8B114 Safari/6534.34.8"
     * ```
     */
    safariUserAgent(): string;

    /**
     * Web address that specifies the location of a resource on the internet.
     * @returns a random url
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.url())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "http://www.forwardtransition.biz/enhance/benchmark"
     * ```
     */
    url(): string;

    /**
     * String sent by a web browser to identify itself when requesting web content.
     * @returns a random user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.userAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (X11; Linux i686) AppleWebKit/5311 (KHTML, like Gecko) Chrome/37.0.834.0 Mobile Safari/5311"
     * ```
     */
    userAgent(): string;

    /**
     * Unique identifier assigned to a user for accessing an account or system.
     * @returns a random username
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.internet.username())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Abshire5538"
     * ```
     */
    username(): string;
  }

  /**
   * Generator to generate language related entries.
   */
  export interface Language {
    /**
     * System of communication using symbols, words, and grammar to convey meaning between individuals.
     * @returns a random language
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.language.language())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Esperanto"
     * ```
     */
    language(): string;

    /**
     * Shortened form of a language's name.
     * @returns a random language abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.language.languageAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "eo"
     * ```
     */
    languageAbbreviation(): string;

    /**
     * Set of guidelines and standards for identifying and representing languages in computing and internet protocols.
     * @returns a random language bcp
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.language.languageBcp())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "he-IL"
     * ```
     */
    languageBcp(): string;

    /**
     * Formal system of instructions used to create software and perform computational tasks.
     * @returns a random programming language
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.language.programmingLanguage())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ceylon"
     * ```
     */
    programmingLanguage(): string;
  }

  /**
   * Generator to generate minecraft related entries.
   */
  export interface Minecraft {
    /**
     * Non-hostile creatures in Minecraft, often used for resources and farming.
     * @returns a random minecraft animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "chicken"
     * ```
     */
    minecraftAnimal(): string;

    /**
     * Component of an armor set in Minecraft, such as a helmet, chestplate, leggings, or boots.
     * @returns a random minecraft armor part
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftArmorPart())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "leggings"
     * ```
     */
    minecraftArmorPart(): string;

    /**
     * Classification system for armor sets in Minecraft, indicating their effectiveness and protection level.
     * @returns a random minecraft armor tier
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftArmorTier())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "leather"
     * ```
     */
    minecraftArmorTier(): string;

    /**
     * Distinctive environmental regions in the game, characterized by unique terrain, vegetation, and weather.
     * @returns a random minecraft biome
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftBiome())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "plain"
     * ```
     */
    minecraftBiome(): string;

    /**
     * Items used to change the color of various in-game objects.
     * @returns a random minecraft dye
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftDye())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "purple"
     * ```
     */
    minecraftDye(): string;

    /**
     * Consumable items in Minecraft that provide nourishment to the player character.
     * @returns a random minecraft food
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftFood())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pufferfish"
     * ```
     */
    minecraftFood(): string;

    /**
     * Powerful hostile creature in the game, often found in challenging dungeons or structures.
     * @returns a random minecraft mob boss
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftMobBoss())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ender dragon"
     * ```
     */
    minecraftMobBoss(): string;

    /**
     * Aggressive creatures in the game that actively attack players when encountered.
     * @returns a random minecraft mob hostile
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftMobHostile())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "blaze"
     * ```
     */
    minecraftMobHostile(): string;

    /**
     * Creature in the game that only becomes hostile if provoked, typically defending itself when attacked.
     * @returns a random minecraft mob neutral
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftMobNeutral())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "dolphin"
     * ```
     */
    minecraftMobNeutral(): string;

    /**
     * Non-aggressive creatures in the game that do not attack players.
     * @returns a random minecraft mob passive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftMobPassive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "axolotl"
     * ```
     */
    minecraftMobPassive(): string;

    /**
     * Naturally occurring minerals found in the game Minecraft, used for crafting purposes.
     * @returns a random minecraft ore
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftOre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "iron"
     * ```
     */
    minecraftOre(): string;

    /**
     * Items in Minecraft designed for specific tasks, including mining, digging, and building.
     * @returns a random minecraft tool
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftTool())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pickaxe"
     * ```
     */
    minecraftTool(): string;

    /**
     * The profession or occupation assigned to a villager character in the game.
     * @returns a random minecraft villager job
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftVillagerJob())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "carpenter"
     * ```
     */
    minecraftVillagerJob(): string;

    /**
     * Measure of a villager's experience and proficiency in their assigned job or profession.
     * @returns a random minecraft villager level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftVillagerLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "novice"
     * ```
     */
    minecraftVillagerLevel(): string;

    /**
     * Designated area or structure in Minecraft where villagers perform their job-related tasks and trading.
     * @returns a random minecraft villager station
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftVillagerStation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lectern"
     * ```
     */
    minecraftVillagerStation(): string;

    /**
     * Tools and items used in Minecraft for combat and defeating hostile mobs.
     * @returns a random minecraft weapon
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftWeapon())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "sword"
     * ```
     */
    minecraftWeapon(): string;

    /**
     * Atmospheric conditions in the game that include rain, thunderstorms, and clear skies, affecting gameplay and ambiance.
     * @returns a random minecraft weather
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftWeather())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "clear"
     * ```
     */
    minecraftWeather(): string;

    /**
     * Natural resource in Minecraft, used for crafting various items and building structures.
     * @returns a random minecraft wood
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.minecraft.minecraftWood())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "oak"
     * ```
     */
    minecraftWood(): string;
  }

  /**
   * Generator to generate movie related entries.
   */
  export interface Movie {
    /**
     * A story told through moving pictures and sound.
     * @returns a random movie
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.movie.movie())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Name":"Sherlock Jr.","Genre":"Music"}
     * ```
     */
    movie(): Record<string, string>;

    /**
     * Category that classifies movies based on common themes, styles, and storytelling approaches.
     * @returns a random movie genre
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.movie.movieGenre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Film-Noir"
     * ```
     */
    movieGenre(): string;

    /**
     * Title or name of a specific film used for identification and reference.
     * @returns a random movie name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.movie.movieName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sherlock Jr."
     * ```
     */
    movieName(): string;
  }

  /**
   * Generator to generate numbers.
   */
  export interface Numbers {
    /**
     * Data type that represents one of two possible values, typically true or false.
     * @returns a random boolean
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.boolean())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * true
     * ```
     */
    boolean(): boolean;

    /**
     * Data type representing floating-point numbers with 32 bits of precision in computing.
     * @returns a random float32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.float32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1.9168120387159532e+38
     * ```
     */
    float32(): number;

    /**
     * Float32 value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random float32 range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.float32Range(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 4.126601219177246
     * ```
     */
    float32Range(min: number, max: number): number;

    /**
     * Data type representing floating-point numbers with 64 bits of precision in computing.
     * @returns a random float64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.float64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1.012641406418422e+308
     * ```
     */
    float64(): number;

    /**
     * Float64 value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random float64 range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.float64Range(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 4.126600960731799
     * ```
     */
    float64Range(min: number, max: number): number;

    /**
     * Hexadecimal representation of an 128-bit unsigned integer.
     * @returns a random hexuint128
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint128())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691402ee58a2330f9c5"
     * ```
     */
    hexUint128(): string;

    /**
     * Hexadecimal representation of an 16-bit unsigned integer.
     * @returns a random hexuint16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b"
     * ```
     */
    hexUint16(): string;

    /**
     * Hexadecimal representation of an 256-bit unsigned integer.
     * @returns a random hexuint256
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint256())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691402ee58a2330f9c54b727953d2379f94d23ea4cdad195b6a"
     * ```
     */
    hexUint256(): string;

    /**
     * Hexadecimal representation of an 32-bit unsigned integer.
     * @returns a random hexuint32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c90"
     * ```
     */
    hexUint32(): string;

    /**
     * Hexadecimal representation of an 64-bit unsigned integer.
     * @returns a random hexuint64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691"
     * ```
     */
    hexUint64(): string;

    /**
     * Hexadecimal representation of an 8-bit unsigned integer.
     * @returns a random hexuint8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.hexUint8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa"
     * ```
     */
    hexUint8(): string;

    /**
     * Signed 16-bit integer, capable of representing values from 32,768 to 32,767.
     * @returns a random int16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.int16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -4595
     * ```
     */
    int16(): number;

    /**
     * Signed 32-bit integer, capable of representing values from -2,147,483,648 to 2,147,483,647.
     * @returns a random int32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.int32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -15831539
     * ```
     */
    int32(): number;

    /**
     * Signed 64-bit integer, capable of representing values from -9,223,372,036,854,775,808 to -9,223,372,036,854,775,807.
     * @returns a random int64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.int64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 5195529898953699000
     * ```
     */
    int64(): number;

    /**
     * Signed 8-bit integer, capable of representing values from -128 to 127.
     * @returns a random int8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.int8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -115
     * ```
     */
    int8(): number;

    /**
     * Integer value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random intrange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.intRange(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 3
     * ```
     */
    intRange(min: number, max: number): number;

    /**
     * Mathematical concept used for counting, measuring, and expressing quantities or values.
     * @param min - Min
     * @param max - Max
     * @returns a random number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.number(-2147483648,2147483647))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -15831539
     * ```
     */
    number(min: number, max: number): number;

    /**
     * Randomly selected value from a slice of int.
     * @param ints - Integers
     * @returns a random random int
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.randomInt([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 14
     * ```
     */
    randomInt(ints: number[]): number;

    /**
     * Randomly selected value from a slice of uint.
     * @param uints - Unsigned Integers
     * @returns a random random uint
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.randomUint([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 14
     * ```
     */
    randomUint(uints: number[]): number;

    /**
     * Shuffles an array of ints.
     * @param ints - Integers
     * @returns a random shuffle ints
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.shuffleInts([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [8,13,14]
     * ```
     */
    shuffleInts(ints: number[]): number[];

    /**
     * Unsigned 16-bit integer, capable of representing values from 0 to 65,535.
     * @returns a random uint16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.uint16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 15082
     * ```
     */
    uint16(): number;

    /**
     * Unsigned 32-bit integer, capable of representing values from 0 to 4,294,967,295.
     * @returns a random uint32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.uint32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 2131652109
     * ```
     */
    uint32(): number;

    /**
     * Unsigned 64-bit integer, capable of representing values from 0 to 18,446,744,073,709,551,615.
     * @returns a random uint64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.uint64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 5195529898953699000
     * ```
     */
    uint64(): number;

    /**
     * Unsigned 8-bit integer, capable of representing values from 0 to 255.
     * @returns a random uint8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.uint8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 234
     * ```
     */
    uint8(): number;

    /**
     * Non-negative integer value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random uintrange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.numbers.uintRange(0,4294967295))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 2131652109
     * ```
     */
    uintRange(min: number, max: number): number;
  }

  /**
   * Generator to generate payment related entries.
   */
  export interface Payment {
    /**
     * A bank account number used for Automated Clearing House transactions and electronic transfers.
     * @returns a random ach account number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.achAccountNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "805388385166"
     * ```
     */
    achAccountNumber(): string;

    /**
     * Unique nine-digit code used in the U.S. for identifying the bank and processing electronic transactions.
     * @returns a random ach routing number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.achRoutingNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "605388385"
     * ```
     */
    achRoutingNumber(): string;

    /**
     * Cryptographic identifier used to receive, store, and send Bitcoin cryptocurrency in a peer-to-peer network.
     * @returns a random bitcoin address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.bitcoinAddress())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1t1xAUWhqY1QsZFAlYm6Z75zxerJ"
     * ```
     */
    bitcoinAddress(): string;

    /**
     * Secret, secure code that allows the owner to access and control their Bitcoin holdings.
     * @returns a random bitcoin private key
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.bitcoinPrivateKey())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "5KgZY1TaSmxpQcUsBAkWXFnidi9UsGRsoQq3dWe4oZz5zrG9VVC"
     * ```
     */
    bitcoinPrivateKey(): string;

    /**
     * Plastic card allowing users to make purchases on credit, with payment due at a later date.
     * @returns a random credit card
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCard())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Type":"Mastercard","Number":"2713883851665706","Exp":"04/32","Cvv":"489"}
     * ```
     */
    creditCard(): Record<string, unknown>;

    /**
     * Three or four-digit security code on a credit card used for online and remote transactions.
     * @returns a random credit card cvv
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardCVV())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "405"
     * ```
     */
    creditCardCVV(): string;

    /**
     * Date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardExp())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "10/27"
     * ```
     */
    creditCardExp(): string;

    /**
     * Month of the date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp month
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardExpMonth())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "07"
     * ```
     */
    creditCardExpMonth(): string;

    /**
     * Year of the date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp year
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardExpYear())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25"
     * ```
     */
    creditCardExpYear(): string;

    /**
     * Unique numerical identifier on a credit card used for making electronic payments and transactions.
     * @param types - Types
     * @param bins - Bins
     * @param gaps - Gaps
     * @returns a random credit card number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardNumber(["none","how","these","keep","trip","congolese","choir","computer","still","far"],["unless","army","party","riches","theirs","instead","here","mine","whichever","that"],false))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0"
     * ```
     */
    creditCardNumber(types: string[], bins: string[], gaps: boolean): string;

    /**
     * Unique numerical identifier on a credit card used for making electronic payments and transactions.
     * @returns a random credit card number formatted
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardNumberFormatted())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "4111-1111-1111-1111"
     * ```
     */
    creditCardNumberFormatted(): string;

    /**
     * Classification of credit cards based on the issuing company.
     * @returns a random credit card type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.creditCardType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mastercard"
     * ```
     */
    creditCardType(): string;

    /**
     * Medium of exchange, often in the form of paper money or coins, used for trade and transactions.
     * @returns a random currency
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.currency())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Short":"VEF","Long":"Venezuela Bolivar"}
     * ```
     */
    currency(): Record<string, string>;

    /**
     * Complete name of a specific currency used for official identification in financial transactions.
     * @returns a random currency long
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.currencyLong())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Venezuela Bolivar"
     * ```
     */
    currencyLong(): string;

    /**
     * Short 3-letter word used to represent a specific currency.
     * @returns a random currency short
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.currencyShort())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "VEF"
     * ```
     */
    currencyShort(): string;

    /**
     * The amount of money or value assigned to a product, service, or asset in a transaction.
     * @param min - Min
     * @param max - Max
     * @returns a random price
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.payment.price(0,1000))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 563.3
     * ```
     */
    price(min: number, max: number): number;
  }

  /**
   * Generator to generate people's personal information.
   */
  export interface Person {
    /**
     * Electronic mail used for sending digital messages and communication over the internet.
     * @returns a random email
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.email())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "josiahthiel@luettgen.biz"
     * ```
     */
    email(): string;

    /**
     * The name given to a person at birth.
     * @returns a random first name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.firstName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Josiah"
     * ```
     */
    firstName(): string;

    /**
     * Classification based on social and cultural norms that identifies an individual.
     * @returns a random gender
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.gender())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "male"
     * ```
     */
    gender(): string;

    /**
     * An activity pursued for leisure and pleasure.
     * @returns a random hobby
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.hobby())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Candy making"
     * ```
     */
    hobby(): string;

    /**
     * The family name or surname of an individual.
     * @returns a random last name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.lastName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Abshire"
     * ```
     */
    lastName(): string;

    /**
     * Name between a person's first name and last name.
     * @returns a random middle name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.middleName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sage"
     * ```
     */
    middleName(): string;

    /**
     * The given and family name of an individual.
     * @returns a random name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.name())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Josiah Thiel"
     * ```
     */
    name(): string;

    /**
     * A title or honorific added before a person's name.
     * @returns a random name prefix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.namePrefix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mr."
     * ```
     */
    namePrefix(): string;

    /**
     * A title or designation added after a person's name.
     * @returns a random name suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.nameSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sr."
     * ```
     */
    nameSuffix(): string;

    /**
     * Personal data, like name and contact details, used for identification and communication.
     * @returns a random person
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.person())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"FirstName":"Josiah","LastName":"Thiel","Gender":"male","SSN":"558821916","Image":"https://picsum.photos/367/273","Hobby":"Winemaking","Job":{"Company":"Headlight","Title":"Administrator","Descriptor":"Chief","Level":"Configuration"},"Address":{"Address":"6992 Inletstad, Las Vegas, Rhode Island 82271","Street":"6992 Inletstad","City":"Las Vegas","State":"Rhode Island","Zip":"82271","Country":"Sweden","Latitude":-75.921372,"Longitude":109.436476},"Contact":{"Phone":"4361943393","Email":"janisbarrows@hessel.net"},"CreditCard":{"Type":"Discover","Number":"4525298222125328","Exp":"01/29","Cvv":"282"}}
     * ```
     */
    person(): Record<string, unknown>;

    /**
     * Numerical sequence used to contact individuals via telephone or mobile devices.
     * @returns a random phone
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.phone())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "7053883851"
     * ```
     */
    phone(): string;

    /**
     * Formatted phone number of a person.
     * @returns a random phone formatted
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.phoneFormatted())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1-053-883-8516"
     * ```
     */
    phoneFormatted(): string;

    /**
     * An institution for formal education and learning.
     * @returns a random school
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.school())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Valley View Private Middle School"
     * ```
     */
    school(): string;

    /**
     * Unique nine-digit identifier used for government and financial purposes in the United States.
     * @returns a random ssn
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.ssn())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "853698829"
     * ```
     */
    ssn(): string;

    /**
     * Randomly split people into teams.
     * @param people - Strings
     * @param teams - Strings
     * @returns a random teams
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.person.teams(["none","how","these","keep","trip","congolese","choir","computer","still","far"],["unless","army","party","riches","theirs","instead","here","mine","whichever","that"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"riches":["choir"],"mine":["how"],"here":["computer"],"whichever":["keep"],"that":["none"],"unless":["these"],"army":["congolese"],"party":["far"],"theirs":["still"],"instead":["trip"]}
     * ```
     */
    teams(people: string[], teams: string[]): Record<string, Array<string>>;
  }

  /**
   * Generator to generate product related entries.
   */
  export interface Product {
    /**
     * An item created for sale or use.
     * @returns a random product
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.product())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Name":"Quartz Teal Scale","Description":"Bravo mirror hundreds his party nobody. Anything wit she from above Chinese those choir toilet as you of other enormously.","Categories":["mobile phones","food and groceries","furniture"],"Price":82.9,"Features":["durable"],"Color":"green","Material":"bronze","UPC":"084020104876"}
     * ```
     */
    product(): Record<string, unknown>;

    /**
     * Classification grouping similar products based on shared characteristics or functions.
     * @returns a random product category
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productCategory())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "mobile phones"
     * ```
     */
    productCategory(): string;

    /**
     * Explanation detailing the features and characteristics of a product.
     * @returns a random product description
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productDescription())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Up brace lung anyway then bravo mirror hundreds his party. Person anything wit she from above Chinese those choir toilet as you."
     * ```
     */
    productDescription(): string;

    /**
     * Specific characteristic of a product that distinguishes it from others products.
     * @returns a random product feature
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productFeature())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "touchscreen"
     * ```
     */
    productFeature(): string;

    /**
     * The substance from which a product is made, influencing its appearance, durability, and properties.
     * @returns a random product material
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productMaterial())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "alloy"
     * ```
     */
    productMaterial(): string;

    /**
     * Distinctive title or label assigned to a product for identification and marketing.
     * @returns a random product name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Stream Gold Robot"
     * ```
     */
    productName(): string;

    /**
     * Standardized barcode used for product identification and tracking in retail and commerce.
     * @returns a random product upc
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.product.productUpc())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "092964558555"
     * ```
     */
    productUpc(): string;
  }

  /**
   * Generator to generate strings.
   */
  export interface Strings {
    /**
     * Numerical symbol used to represent numbers.
     * @returns a random digit
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.digit())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0"
     * ```
     */
    digit(): string;

    /**
     * string of length N consisting of ASCII digits.
     * @param count - Count
     * @returns a random digitn
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.digitN(3))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "005"
     * ```
     */
    digitN(count: number): string;

    /**
     * Character or symbol from the American Standard Code for Information Interchange (ASCII) character set.
     * @returns a random letter
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.letter())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "W"
     * ```
     */
    letter(): string;

    /**
     * ASCII string with length N.
     * @param count - Count
     * @returns a random lettern
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.letterN(3))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "WCp"
     * ```
     */
    letterN(count: number): string;

    /**
     * Replace ? with random generated letters.
     * @param str - String
     * @returns a random lexify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.lexify("none"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    lexify(str: string): string;

    /**
     * Replace # with random numerical values.
     * @param str - String
     * @returns a random numerify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.numerify("none"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    numerify(str: string): string;

    /**
     * Return a random string from a string array.
     * @param strs - Strings
     * @returns a random random string
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.randomString(["none","how","these","keep","trip","congolese","choir","computer","still","far"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    randomString(strs: string[]): string;

    /**
     * Shuffle an array of strings.
     * @param strs - Strings
     * @returns a random shuffle strings
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.shuffleStrings(["none","how","these","keep","trip","congolese","choir","computer","still","far"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * ["these","congolese","far","choir","still","trip","computer","how","keep","none"]
     * ```
     */
    shuffleStrings(strs: string[]): string[];

    /**
     * 128-bit identifier used to uniquely identify objects or entities in computer systems.
     * @returns a random uuid
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.strings.uuid())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ea6ab1ab-f06c-4990-835d-e628b7e659e1"
     * ```
     */
    uuid(): string;
  }

  /**
   * Generator to generate time and date.
   */
  export interface Time {
    /**
     * Representation of a specific day, month, and year, often used for chronological reference.
     * @param format - Format
     * @returns a random date
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.date("RFC3339"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1952-06-14T22:21:28Z"
     * ```
     */
    date(format: string): string;

    /**
     * Random date between two ranges.
     * @param startdate - Start Date
     * @param enddate - End Date
     * @param format - Format
     * @returns a random daterange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.dateRange("1970-01-01","2024-03-13","yyyy-MM-dd"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2008-04-06"
     * ```
     */
    dateRange(startdate: string, enddate: string, format: string): string;

    /**
     * 24-hour period equivalent to one rotation of Earth on its axis.
     * @returns a random day
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.day())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 22
     * ```
     */
    day(): number;

    /**
     * Date that has occurred after the current moment in time.
     * @returns a random futuretime
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.futureTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2024-12-18T19:55:55.75608953+01:00"
     * ```
     */
    futureTime(): string;

    /**
     * Unit of time equal to 60 minutes.
     * @returns a random hour
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.hour())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 21
     * ```
     */
    hour(): number;

    /**
     * Unit of time equal to 60 seconds.
     * @returns a random minute
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.minute())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 9
     * ```
     */
    minute(): number;

    /**
     * Division of the year, typically 30 or 31 days long.
     * @returns a random month
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.month())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 10
     * ```
     */
    month(): string;

    /**
     * String Representation of a month name.
     * @returns a random month string
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.monthString())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "October"
     * ```
     */
    monthString(): string;

    /**
     * Unit of time equal to One billionth (10^-9) of a second.
     * @returns a random nanosecond
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.nanosecond())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 953698829
     * ```
     */
    nanosecond(): number;

    /**
     * Date that has occurred before the current moment in time.
     * @returns a random pasttime
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.pastTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2024-12-17T23:55:55.756548281+01:00"
     * ```
     */
    pastTime(): string;

    /**
     * Unit of time equal to 1/60th of a minute.
     * @returns a random second
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.second())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 9
     * ```
     */
    second(): number;

    /**
     * Region where the same standard time is used, based on longitudinal divisions of the Earth.
     * @returns a random timezone
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.timezone())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tonga Standard Time"
     * ```
     */
    timezone(): string;

    /**
     * Abbreviated 3-letter word of a timezone.
     * @returns a random timezone abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.timezoneAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "TST"
     * ```
     */
    timezoneAbbreviation(): string;

    /**
     * Full name of a timezone.
     * @returns a random timezone full
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.timezoneFull())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "(UTC+13:00) Nuku'alofa"
     * ```
     */
    timezoneFull(): string;

    /**
     * The difference in hours from Coordinated Universal Time (UTC) for a specific region.
     * @returns a random timezone offset
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.timezoneOffset())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 13
     * ```
     */
    timezoneOffset(): number;

    /**
     * Geographic area sharing the same standard time.
     * @returns a random timezone region
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.timezoneRegion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Asia/Manila"
     * ```
     */
    timezoneRegion(): string;

    /**
     * Day of the week excluding the weekend.
     * @returns a random weekday
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.weekday())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sunday"
     * ```
     */
    weekday(): string;

    /**
     * Period of 365 days, the time Earth takes to orbit the Sun.
     * @returns a random year
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.time.year())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1979
     * ```
     */
    year(): number;
  }

  /**
   * Generator to generate words and sentences.
   */
  export interface Word {
    /**
     * Verb Indicating a physical or mental action.
     * @returns a random action verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.actionVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "smell"
     * ```
     */
    actionVerb(): string;

    /**
     * Word describing or modifying a noun.
     * @returns a random adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "brave"
     * ```
     */
    adjective(): string;

    /**
     * Word that modifies verbs, adjectives, or other adverbs.
     * @returns a random adverb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quickly"
     * ```
     */
    adverb(): string;

    /**
     * Adverb that indicates the degree or intensity of an action or adjective.
     * @returns a random adverb degree
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbDegree())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pretty"
     * ```
     */
    adverbDegree(): string;

    /**
     * Adverb that specifies how often an action occurs with a clear frequency.
     * @returns a random adverb frequency definite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbFrequencyDefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "annually"
     * ```
     */
    adverbFrequencyDefinite(): string;

    /**
     * Adverb that specifies how often an action occurs without specifying a particular frequency.
     * @returns a random adverb frequency indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbFrequencyIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "rarely"
     * ```
     */
    adverbFrequencyIndefinite(): string;

    /**
     * Adverb that describes how an action is performed.
     * @returns a random adverb manner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbManner())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "sleepily"
     * ```
     */
    adverbManner(): string;

    /**
     * Phrase that modifies a verb, adjective, or another adverb, providing additional information..
     * @returns a random adverb phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "too cheerfully"
     * ```
     */
    adverbPhrase(): string;

    /**
     * Adverb that indicates the location or direction of an action.
     * @returns a random adverb place
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbPlace())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "east"
     * ```
     */
    adverbPlace(): string;

    /**
     * Adverb that specifies the exact time an action occurs.
     * @returns a random adverb time definite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbTimeDefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "now"
     * ```
     */
    adverbTimeDefinite(): string;

    /**
     * Adverb that gives a general or unspecified time frame.
     * @returns a random adverb time indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.adverbTimeIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "late"
     * ```
     */
    adverbTimeIndefinite(): string;

    /**
     * Statement or remark expressing an opinion, observation, or reaction.
     * @returns a random comment
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.comment())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wow"
     * ```
     */
    comment(): string;

    /**
     * Word used to connect words or sentences.
     * @returns a random connective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for another"
     * ```
     */
    connective(): string;

    /**
     * Connective word used to indicate a cause-and-effect relationship between events or actions.
     * @returns a random connective casual
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveCasual())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accordingly"
     * ```
     */
    connectiveCasual(): string;

    /**
     * Connective word used to indicate a comparison between two or more things.
     * @returns a random connective comparitive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveComparitive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "yet"
     * ```
     */
    connectiveComparitive(): string;

    /**
     * Connective word used to express dissatisfaction or complaints about a situation.
     * @returns a random connective complaint
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveComplaint())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for example"
     * ```
     */
    connectiveComplaint(): string;

    /**
     * Connective word used to provide examples or illustrations of a concept or idea.
     * @returns a random connective examplify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveExamplify())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accordingly"
     * ```
     */
    connectiveExamplify(): string;

    /**
     * Connective word used to list or enumerate items or examples.
     * @returns a random connective listing
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveListing())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for another"
     * ```
     */
    connectiveListing(): string;

    /**
     * Connective word used to indicate a temporal relationship between events or actions.
     * @returns a random connective time
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.connectiveTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "until then"
     * ```
     */
    connectiveTime(): string;

    /**
     * Adjective used to point out specific things.
     * @returns a random demonstrative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.demonstrativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    demonstrativeAdjective(): string;

    /**
     * Adjective that provides detailed characteristics about a noun.
     * @returns a random descriptive adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.descriptiveAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "elated"
     * ```
     */
    descriptiveAdjective(): string;

    /**
     * Auxiliary verb that helps the main verb complete the sentence.
     * @returns a random helping verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.helpingVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "are"
     * ```
     */
    helpingVerb(): string;

    /**
     * Adjective describing a non-specific noun.
     * @returns a random indefinite adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.indefiniteAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "somebody"
     * ```
     */
    indefiniteAdjective(): string;

    /**
     * Word expressing emotion.
     * @returns a random interjection
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.interjection())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wow"
     * ```
     */
    interjection(): string;

    /**
     * Adjective used to ask questions.
     * @returns a random interrogative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.interrogativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "what"
     * ```
     */
    interrogativeAdjective(): string;

    /**
     * Verb that does not require a direct object to complete its meaning.
     * @returns a random intransitive verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.intransitiveVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "skip"
     * ```
     */
    intransitiveVerb(): string;

    /**
     * Verb that Connects the subject of a sentence to a subject complement.
     * @returns a random linking verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.linkingVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "had"
     * ```
     */
    linkingVerb(): string;

    /**
     * Paragraph of the Lorem Ipsum placeholder text used in design and publishing.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random lorem ipsum paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.loremIpsumParagraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Accusamus et voluptatum voluptatem nisi. Nostrum atque molestias reprehenderit alias.<br />Reiciendis ut eos ut ad. Ea magni recusandae id fuga."
     * ```
     */
    loremIpsumParagraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * Sentence of the Lorem Ipsum placeholder text used in design and publishing.
     * @param wordcount - Word Count
     * @returns a random lorem ipsum sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.loremIpsumSentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Accusamus et voluptatum voluptatem nisi."
     * ```
     */
    loremIpsumSentence(wordcount: number): string;

    /**
     * Word of the Lorem Ipsum placeholder text used in design and publishing.
     * @returns a random lorem ipsum word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.loremIpsumWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accusamus"
     * ```
     */
    loremIpsumWord(): string;

    /**
     * Person, place, thing, or idea, named or referred to in a sentence.
     * @returns a random noun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.noun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "hand"
     * ```
     */
    noun(): string;

    /**
     * Ideas, qualities, or states that cannot be perceived with the five senses.
     * @returns a random noun abstract
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounAbstract())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "philosophy"
     * ```
     */
    nounAbstract(): string;

    /**
     * Group of animals, like a 'pack' of wolves or a 'flock' of birds.
     * @returns a random noun collective animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounCollectiveAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "school"
     * ```
     */
    nounCollectiveAnimal(): string;

    /**
     * Group of people or things regarded as a unit.
     * @returns a random noun collective people
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounCollectivePeople())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "bevy"
     * ```
     */
    nounCollectivePeople(): string;

    /**
     * Group of objects or items, such as a 'bundle' of sticks or a 'cluster' of grapes.
     * @returns a random noun collective thing
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounCollectiveThing())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wad"
     * ```
     */
    nounCollectiveThing(): string;

    /**
     * General name for people, places, or things, not specific or unique.
     * @returns a random noun common
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounCommon())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "company"
     * ```
     */
    nounCommon(): string;

    /**
     * Names for physical entities experienced through senses like sight, touch, smell, or taste.
     * @returns a random noun concrete
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounConcrete())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "train"
     * ```
     */
    nounConcrete(): string;

    /**
     * Items that can be counted individually.
     * @returns a random noun countable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounCountable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "weekend"
     * ```
     */
    nounCountable(): string;

    /**
     * Word that introduces a noun and identifies it as a noun.
     * @returns a random noun determiner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounDeterminer())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "this"
     * ```
     */
    nounDeterminer(): string;

    /**
     * Phrase with a noun as its head, functions within sentence like a noun.
     * @returns a random noun phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "a brave fuel"
     * ```
     */
    nounPhrase(): string;

    /**
     * Specific name for a particular person, place, or organization.
     * @returns a random noun proper
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounProper())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Rowan Atkinson"
     * ```
     */
    nounProper(): string;

    /**
     * Items that can't be counted individually.
     * @returns a random noun uncountable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.nounUncountable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "butter"
     * ```
     */
    nounUncountable(): string;

    /**
     * Distinct section of writing covering a single theme, composed of multiple sentences.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.paragraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Quickly up brace lung anyway. Then bravo mirror hundreds his.<br />Party nobody person anything wit. She from above Chinese those."
     * ```
     */
    paragraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * A small group of words standing together.
     * @returns a random phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.phrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "many thanks"
     * ```
     */
    phrase(): string;

    /**
     * Adjective indicating ownership or possession.
     * @returns a random possessive adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.possessiveAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "his"
     * ```
     */
    possessiveAdjective(): string;

    /**
     * Words used to express the relationship of a noun or pronoun to other words in a sentence.
     * @returns a random preposition
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.preposition())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "out"
     * ```
     */
    preposition(): string;

    /**
     * Preposition that can be formed by combining two or more prepositions.
     * @returns a random preposition compound
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.prepositionCompound())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "apart from"
     * ```
     */
    prepositionCompound(): string;

    /**
     * Two-word combination preposition, indicating a complex relation.
     * @returns a random preposition double
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.prepositionDouble())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "outside of"
     * ```
     */
    prepositionDouble(): string;

    /**
     * Phrase starting with a preposition, showing relation between elements in a sentence..
     * @returns a random preposition phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.prepositionPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "of a fuel"
     * ```
     */
    prepositionPhrase(): string;

    /**
     * Single-word preposition showing relationships between 2 parts of a sentence.
     * @returns a random preposition simple
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.prepositionSimple())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "of"
     * ```
     */
    prepositionSimple(): string;

    /**
     * Word used in place of a noun to avoid repetition.
     * @returns a random pronoun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronoun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    pronoun(): string;

    /**
     * Pronoun that points out specific people or things.
     * @returns a random pronoun demonstrative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounDemonstrative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    pronounDemonstrative(): string;

    /**
     * Pronoun that does not refer to a specific person or thing.
     * @returns a random pronoun indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "anyone"
     * ```
     */
    pronounIndefinite(): string;

    /**
     * Pronoun used to ask questions.
     * @returns a random pronoun interrogative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounInterrogative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "who"
     * ```
     */
    pronounInterrogative(): string;

    /**
     * Pronoun used as the object of a verb or preposition.
     * @returns a random pronoun object
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounObject())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "you"
     * ```
     */
    pronounObject(): string;

    /**
     * Pronoun referring to a specific persons or things.
     * @returns a random pronoun personal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounPersonal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "you"
     * ```
     */
    pronounPersonal(): string;

    /**
     * Pronoun indicating ownership or belonging.
     * @returns a random pronoun possessive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounPossessive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "mine"
     * ```
     */
    pronounPossessive(): string;

    /**
     * Pronoun referring back to the subject of the sentence.
     * @returns a random pronoun reflective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounReflective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "herself"
     * ```
     */
    pronounReflective(): string;

    /**
     * Pronoun that introduces a clause, referring back to a noun or pronoun.
     * @returns a random pronoun relative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.pronounRelative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "that"
     * ```
     */
    pronounRelative(): string;

    /**
     * Adjective derived from a proper noun, often used to describe nationality or origin.
     * @returns a random proper adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.properAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Confucian"
     * ```
     */
    properAdjective(): string;

    /**
     * Adjective that indicates the quantity or amount of something.
     * @returns a random quantitative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.quantitativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "several"
     * ```
     */
    quantitativeAdjective(): string;

    /**
     * Statement formulated to inquire or seek clarification.
     * @returns a random question
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.question())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Forage pinterest direct trade pug skateboard food truck flannel cold-pressed?"
     * ```
     */
    question(): string;

    /**
     * Direct repetition of someone else's words.
     * @returns a random quote
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.quote())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "\"Forage pinterest direct trade pug skateboard food truck flannel cold-pressed.\" - Lukas Ledner"
     * ```
     */
    quote(): string;

    /**
     * Set of words expressing a statement, question, exclamation, or command.
     * @param wordcount - Word Count
     * @returns a random sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.sentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Quickly up brace lung anyway."
     * ```
     */
    sentence(wordcount: number): string;

    /**
     * Group of words that expresses a complete thought.
     * @returns a random simple sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.simpleSentence())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "A brave fuel enormously beautifully stack easy day less badly in a bunch."
     * ```
     */
    simpleSentence(): string;

    /**
     * Verb that requires a direct object to complete its meaning.
     * @returns a random transitive verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.transitiveVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "bother"
     * ```
     */
    transitiveVerb(): string;

    /**
     * Word expressing an action, event or state.
     * @returns a random verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.verb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "dig"
     * ```
     */
    verb(): string;

    /**
     * Phrase that Consists of a verb and its modifiers, expressing an action or state.
     * @returns a random verb phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.verbPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "cheerfully cry enormously beautifully with easy day less badly"
     * ```
     */
    verbPhrase(): string;

    /**
     * Basic unit of language representing a concept or thing, consisting of letters and having meaning.
     * @returns a random word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.word.word())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quickly"
     * ```
     */
    word(): string;
  }

  /**
   * Generator with all generator functions for convenient use.
   */
  export interface Zen {
    /**
     * A bank account number used for Automated Clearing House transactions and electronic transfers.
     * @returns a random ach account number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.achAccountNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "805388385166"
     * ```
     */
    achAccountNumber(): string;

    /**
     * Unique nine-digit code used in the U.S. for identifying the bank and processing electronic transactions.
     * @returns a random ach routing number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.achRoutingNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "605388385"
     * ```
     */
    achRoutingNumber(): string;

    /**
     * Verb Indicating a physical or mental action.
     * @returns a random action verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.actionVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "smell"
     * ```
     */
    actionVerb(): string;

    /**
     * Residential location including street, city, state, country and postal code.
     * @returns a random address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Address":"53883 Villageborough, San Bernardino, Kentucky 56992","Street":"53883 Villageborough","City":"San Bernardino","State":"Kentucky","Zip":"56992","Country":"United States of America","Latitude":11.29359,"Longitude":-145.577493}
     * ```
     */
    address(): Record<string, unknown>;

    /**
     * Word describing or modifying a noun.
     * @returns a random adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "brave"
     * ```
     */
    adjective(): string;

    /**
     * Word that modifies verbs, adjectives, or other adverbs.
     * @returns a random adverb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quickly"
     * ```
     */
    adverb(): string;

    /**
     * Adverb that indicates the degree or intensity of an action or adjective.
     * @returns a random adverb degree
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbDegree())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pretty"
     * ```
     */
    adverbDegree(): string;

    /**
     * Adverb that specifies how often an action occurs with a clear frequency.
     * @returns a random adverb frequency definite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbFrequencyDefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "annually"
     * ```
     */
    adverbFrequencyDefinite(): string;

    /**
     * Adverb that specifies how often an action occurs without specifying a particular frequency.
     * @returns a random adverb frequency indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbFrequencyIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "rarely"
     * ```
     */
    adverbFrequencyIndefinite(): string;

    /**
     * Adverb that describes how an action is performed.
     * @returns a random adverb manner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbManner())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "sleepily"
     * ```
     */
    adverbManner(): string;

    /**
     * Phrase that modifies a verb, adjective, or another adverb, providing additional information..
     * @returns a random adverb phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "too cheerfully"
     * ```
     */
    adverbPhrase(): string;

    /**
     * Adverb that indicates the location or direction of an action.
     * @returns a random adverb place
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbPlace())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "east"
     * ```
     */
    adverbPlace(): string;

    /**
     * Adverb that specifies the exact time an action occurs.
     * @returns a random adverb time definite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbTimeDefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "now"
     * ```
     */
    adverbTimeDefinite(): string;

    /**
     * Adverb that gives a general or unspecified time frame.
     * @returns a random adverb time indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.adverbTimeIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "late"
     * ```
     */
    adverbTimeIndefinite(): string;

    /**
     * Living creature with the ability to move, eat, and interact with its environment.
     * @returns a random animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.animal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "crow"
     * ```
     */
    animal(): string;

    /**
     * Type of animal, such as mammals, birds, reptiles, etc..
     * @returns a random animal type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.animalType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "amphibians"
     * ```
     */
    animalType(): string;

    /**
     * Person or group creating and developing an application.
     * @returns a random app author
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.appAuthor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Wendell Luettgen"
     * ```
     */
    appAuthor(): string;

    /**
     * Software program designed for a specific purpose or task on a computer or mobile device.
     * @returns a random app name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.appName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hillbe"
     * ```
     */
    appName(): string;

    /**
     * Particular release of an application in Semantic Versioning format.
     * @returns a random app version
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.appVersion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "5.3.20"
     * ```
     */
    appVersion(): string;

    /**
     * Measures the alcohol content in beer.
     * @returns a random beer alcohol
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerAlcohol())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "6.5%"
     * ```
     */
    beerAlcohol(): string;

    /**
     * Scale indicating the concentration of extract in worts.
     * @returns a random beer blg
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerBlg())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "13.4°Blg"
     * ```
     */
    beerBlg(): string;

    /**
     * The flower used in brewing to add flavor, aroma, and bitterness to beer.
     * @returns a random beer hop
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerHop())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Nugget"
     * ```
     */
    beerHop(): string;

    /**
     * Scale measuring bitterness of beer from hops.
     * @returns a random beer ibu
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerIbu())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "80 IBU"
     * ```
     */
    beerIbu(): string;

    /**
     * Processed barley or other grains, provides sugars for fermentation and flavor to beer.
     * @returns a random beer malt
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerMalt())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Roasted barley"
     * ```
     */
    beerMalt(): string;

    /**
     * Specific brand or variety of beer.
     * @returns a random beer name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "90 Minute IPA"
     * ```
     */
    beerName(): string;

    /**
     * Distinct characteristics and flavors of beer.
     * @returns a random beer style
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerStyle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "English Brown Ale"
     * ```
     */
    beerStyle(): string;

    /**
     * Microorganism used in brewing to ferment sugars, producing alcohol and carbonation in beer.
     * @returns a random beer yeast
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.beerYeast())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2035 - American Lager"
     * ```
     */
    beerYeast(): string;

    /**
     * Distinct species of birds.
     * @returns a random bird
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bird())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lovebird"
     * ```
     */
    bird(): string;

    /**
     * Cryptographic identifier used to receive, store, and send Bitcoin cryptocurrency in a peer-to-peer network.
     * @returns a random bitcoin address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bitcoinAddress())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1t1xAUWhqY1QsZFAlYm6Z75zxerJ"
     * ```
     */
    bitcoinAddress(): string;

    /**
     * Secret, secure code that allows the owner to access and control their Bitcoin holdings.
     * @returns a random bitcoin private key
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bitcoinPrivateKey())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "5KgZY1TaSmxpQcUsBAkWXFnidi9UsGRsoQq3dWe4oZz5zrG9VVC"
     * ```
     */
    bitcoinPrivateKey(): string;

    /**
     * Brief description or summary of a company's purpose, products, or services.
     * @returns a random blurb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.blurb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Pride"
     * ```
     */
    blurb(): string;

    /**
     * Written or printed work consisting of pages bound together, covering various subjects or stories.
     * @returns a random book
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.book())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Title":"The Brothers Karamazov","Author":"Albert Camus","Genre":"Urban"}
     * ```
     */
    book(): Record<string, string>;

    /**
     * The individual who wrote or created the content of a book.
     * @returns a random book author
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bookAuthor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Edgar Allan Poe"
     * ```
     */
    bookAuthor(): string;

    /**
     * Category or type of book defined by its content, style, or form.
     * @returns a random book genre
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bookGenre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Erotic"
     * ```
     */
    bookGenre(): string;

    /**
     * The specific name given to a book.
     * @returns a random book title
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bookTitle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "The Brothers Karamazov"
     * ```
     */
    bookTitle(): string;

    /**
     * Data type that represents one of two possible values, typically true or false.
     * @returns a random boolean
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.boolean())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * true
     * ```
     */
    boolean(): boolean;

    /**
     * First meal of the day, typically eaten in the morning.
     * @returns a random breakfast
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.breakfast())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ham omelet deluxe"
     * ```
     */
    breakfast(): string;

    /**
     * Random bs company word.
     * @returns a random bs
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.bs())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "24-7"
     * ```
     */
    bs(): string;

    /**
     * Trendy or overused term often used in business to sound impressive.
     * @returns a random buzzword
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.buzzword())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Reverse-engineered"
     * ```
     */
    buzzword(): string;

    /**
     * Wheeled motor vehicle used for transportation.
     * @returns a random car
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.car())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Type":"Passenger car compact","Fuel":"CNG","Transmission":"Automatic","Brand":"Daewoo","Model":"Thunderbird","Year":1905}
     * ```
     */
    car(): Record<string, unknown>;

    /**
     * Type of energy source a car uses.
     * @returns a random car fuel type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.carFuelType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ethanol"
     * ```
     */
    carFuelType(): string;

    /**
     * Company or brand that manufactures and designs cars.
     * @returns a random car maker
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.carMaker())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Lancia"
     * ```
     */
    carMaker(): string;

    /**
     * Specific design or version of a car produced by a manufacturer.
     * @returns a random car model
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.carModel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tucson 4wd"
     * ```
     */
    carModel(): string;

    /**
     * Mechanism a car uses to transmit power from the engine to the wheels.
     * @returns a random car transmission type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.carTransmissionType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Manual"
     * ```
     */
    carTransmissionType(): string;

    /**
     * Classification of cars based on size, use, or body style.
     * @returns a random car type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.carType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Passenger car compact"
     * ```
     */
    carType(): string;

    /**
     * Various breeds that define different cats.
     * @returns a random cat
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.cat())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Toyger"
     * ```
     */
    cat(): string;

    /**
     * Famous person known for acting in films, television, or theater.
     * @returns a random celebrity actor
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.celebrityActor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ben Affleck"
     * ```
     */
    celebrityActor(): string;

    /**
     * High-profile individual known for significant achievements in business or entrepreneurship.
     * @returns a random celebrity business
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.celebrityBusiness())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Larry Ellison"
     * ```
     */
    celebrityBusiness(): string;

    /**
     * Famous athlete known for achievements in a particular sport.
     * @returns a random celebrity sport
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.celebritySport())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Greg Lemond"
     * ```
     */
    celebritySport(): string;

    /**
     * The specific identification string sent by the Google Chrome web browser when making requests on the internet.
     * @returns a random chrome user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.chromeUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (X11; Linux i686) AppleWebKit/5340 (KHTML, like Gecko) Chrome/40.0.816.0 Mobile Safari/5340"
     * ```
     */
    chromeUserAgent(): string;

    /**
     * Part of a country with significant population, often a central hub for culture and commerce.
     * @returns a random city
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.city())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hialeah"
     * ```
     */
    city(): string;

    /**
     * Hue seen by the eye, returns the name of the color like red or blue.
     * @returns a random color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.color())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "MediumVioletRed"
     * ```
     */
    color(): string;

    /**
     * Statement or remark expressing an opinion, observation, or reaction.
     * @returns a random comment
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.comment())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wow"
     * ```
     */
    comment(): string;

    /**
     * Designated official name of a business or organization.
     * @returns a random company
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.company())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Xatori"
     * ```
     */
    company(): string;

    /**
     * Suffix at the end of a company name, indicating business structure, like 'Inc.' or 'LLC'.
     * @returns a random company suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.companySuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "LLC"
     * ```
     */
    companySuffix(): string;

    /**
     * Word used to connect words or sentences.
     * @returns a random connective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for another"
     * ```
     */
    connective(): string;

    /**
     * Connective word used to indicate a cause-and-effect relationship between events or actions.
     * @returns a random connective casual
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveCasual())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accordingly"
     * ```
     */
    connectiveCasual(): string;

    /**
     * Connective word used to indicate a comparison between two or more things.
     * @returns a random connective comparitive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveComparitive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "yet"
     * ```
     */
    connectiveComparitive(): string;

    /**
     * Connective word used to express dissatisfaction or complaints about a situation.
     * @returns a random connective complaint
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveComplaint())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for example"
     * ```
     */
    connectiveComplaint(): string;

    /**
     * Connective word used to provide examples or illustrations of a concept or idea.
     * @returns a random connective examplify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveExamplify())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accordingly"
     * ```
     */
    connectiveExamplify(): string;

    /**
     * Connective word used to list or enumerate items or examples.
     * @returns a random connective listing
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveListing())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "for another"
     * ```
     */
    connectiveListing(): string;

    /**
     * Connective word used to indicate a temporal relationship between events or actions.
     * @returns a random connective time
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.connectiveTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "until then"
     * ```
     */
    connectiveTime(): string;

    /**
     * Nation with its own government and defined territory.
     * @returns a random country
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.country())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Togo"
     * ```
     */
    country(): string;

    /**
     * Shortened 2-letter form of a country's name.
     * @returns a random country abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.countryAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "TG"
     * ```
     */
    countryAbbreviation(): string;

    /**
     * Plastic card allowing users to make purchases on credit, with payment due at a later date.
     * @returns a random credit card
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCard())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Type":"Mastercard","Number":"2713883851665706","Exp":"04/32","Cvv":"489"}
     * ```
     */
    creditCard(): Record<string, unknown>;

    /**
     * Three or four-digit security code on a credit card used for online and remote transactions.
     * @returns a random credit card cvv
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardCVV())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "405"
     * ```
     */
    creditCardCVV(): string;

    /**
     * Date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardExp())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "10/27"
     * ```
     */
    creditCardExp(): string;

    /**
     * Month of the date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp month
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardExpMonth())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "07"
     * ```
     */
    creditCardExpMonth(): string;

    /**
     * Year of the date when a credit card becomes invalid and cannot be used for transactions.
     * @returns a random credit card exp year
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardExpYear())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25"
     * ```
     */
    creditCardExpYear(): string;

    /**
     * Unique numerical identifier on a credit card used for making electronic payments and transactions.
     * @param types - Types
     * @param bins - Bins
     * @param gaps - Gaps
     * @returns a random credit card number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardNumber(["none","how","these","keep","trip","congolese","choir","computer","still","far"],["unless","army","party","riches","theirs","instead","here","mine","whichever","that"],false))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0"
     * ```
     */
    creditCardNumber(types: string[], bins: string[], gaps: boolean): string;

    /**
     * Unique numerical identifier on a credit card used for making electronic payments and transactions.
     * @returns a random credit card number formatted
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardNumberFormatted())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "4111-1111-1111-1111"
     * ```
     */
    creditCardNumberFormatted(): string;

    /**
     * Classification of credit cards based on the issuing company.
     * @returns a random credit card type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.creditCardType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mastercard"
     * ```
     */
    creditCardType(): string;

    /**
     * Medium of exchange, often in the form of paper money or coins, used for trade and transactions.
     * @returns a random currency
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.currency())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Short":"VEF","Long":"Venezuela Bolivar"}
     * ```
     */
    currency(): Record<string, string>;

    /**
     * Complete name of a specific currency used for official identification in financial transactions.
     * @returns a random currency long
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.currencyLong())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Venezuela Bolivar"
     * ```
     */
    currencyLong(): string;

    /**
     * Short 3-letter word used to represent a specific currency.
     * @returns a random currency short
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.currencyShort())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "VEF"
     * ```
     */
    currencyShort(): string;

    /**
     * Unique identifier for securities, especially bonds, in the United States and Canada.
     * @returns a random cusip
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.cusip())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "S4BL2MVY6"
     * ```
     */
    cusip(): string;

    /**
     * A problem or issue encountered while accessing or managing a database.
     * @returns a random database error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.databaseError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    databaseError(): string;

    /**
     * Representation of a specific day, month, and year, often used for chronological reference.
     * @param format - Format
     * @returns a random date
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.date("RFC3339"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1959-04-03T13:46:53Z"
     * ```
     */
    date(format: string): string;

    /**
     * Random date between two ranges.
     * @param startdate - Start Date
     * @param enddate - End Date
     * @param format - Format
     * @returns a random daterange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.dateRange("1970-01-01","2024-03-13","yyyy-MM-dd"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1979-05-06"
     * ```
     */
    dateRange(startdate: string, enddate: string, format: string): string;

    /**
     * 24-hour period equivalent to one rotation of Earth on its axis.
     * @returns a random day
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.day())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 22
     * ```
     */
    day(): number;

    /**
     * Adjective used to point out specific things.
     * @returns a random demonstrative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.demonstrativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    demonstrativeAdjective(): string;

    /**
     * Adjective that provides detailed characteristics about a noun.
     * @returns a random descriptive adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.descriptiveAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "elated"
     * ```
     */
    descriptiveAdjective(): string;

    /**
     * Sweet treat often enjoyed after a meal.
     * @returns a random dessert
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.dessert())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Lindas bloodshot eyeballs"
     * ```
     */
    dessert(): string;

    /**
     * Small, cube-shaped objects used in games of chance for random outcomes.
     * @param numdice - Number of Dice
     * @param sides - Number of Sides
     * @returns a random dice
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.dice(1,[5,4,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [5]
     * ```
     */
    dice(numdice: number, sides: number[]): number[];

    /**
     * Numerical symbol used to represent numbers.
     * @returns a random digit
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.digit())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0"
     * ```
     */
    digit(): string;

    /**
     * string of length N consisting of ASCII digits.
     * @param count - Count
     * @returns a random digitn
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.digitN(3))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "005"
     * ```
     */
    digitN(count: number): string;

    /**
     * Evening meal, typically the day's main and most substantial meal.
     * @returns a random dinner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.dinner())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Asian broccoli salad"
     * ```
     */
    dinner(): string;

    /**
     * Various breeds that define different dogs.
     * @returns a random dog
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.dog())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Staffordshire Bullterrier"
     * ```
     */
    dog(): string;

    /**
     * Human-readable web address used to identify websites on the internet.
     * @returns a random domain name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.domainName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "internalenhance.org"
     * ```
     */
    domainName(): string;

    /**
     * The part of a domain name that comes after the last dot, indicating its type or purpose.
     * @returns a random domain suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.domainSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "info"
     * ```
     */
    domainSuffix(): string;

    /**
     * Liquid consumed for hydration, pleasure, or nutritional benefits.
     * @returns a random drink
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.drink())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Water"
     * ```
     */
    drink(): string;

    /**
     * Electronic mail used for sending digital messages and communication over the internet.
     * @returns a random email
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.email())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "josiahthiel@luettgen.biz"
     * ```
     */
    email(): string;

    /**
     * Digital symbol expressing feelings or ideas in text messages and online chats.
     * @returns a random emoji
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.emoji())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "🐮"
     * ```
     */
    emoji(): string;

    /**
     * Alternative name or keyword used to represent a specific emoji in text or code.
     * @returns a random emoji alias
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.emojiAlias())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "slovakia"
     * ```
     */
    emojiAlias(): string;

    /**
     * Group or classification of emojis based on their common theme or use, like 'smileys' or 'animals'.
     * @returns a random emoji category
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.emojiCategory())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Smileys & Emotion"
     * ```
     */
    emojiCategory(): string;

    /**
     * Brief explanation of the meaning or emotion conveyed by an emoji.
     * @returns a random emoji description
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.emojiDescription())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "disguised face"
     * ```
     */
    emojiDescription(): string;

    /**
     * Label or keyword associated with an emoji to categorize or search for it easily.
     * @returns a random emoji tag
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.emojiTag())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lick"
     * ```
     */
    emojiTag(): string;

    /**
     * Message displayed by a computer or software when a problem or mistake is encountered.
     * @returns a random error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.error())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    error(): string;

    /**
     * Various categories conveying details about encountered errors.
     * @returns a random error object word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.errorObjectWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    errorObjectWord(): string;

    /**
     * Animal name commonly found on a farm.
     * @returns a random farm animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.farmAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Cow"
     * ```
     */
    farmAnimal(): string;

    /**
     * Suffix appended to a filename indicating its format or type.
     * @returns a random file extension
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.fileExtension())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "max"
     * ```
     */
    fileExtension(): string;

    /**
     * Defines file format and nature for browsers and email clients using standardized identifiers.
     * @returns a random file mime type
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.fileMimeType())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "text/html"
     * ```
     */
    fileMimeType(): string;

    /**
     * The specific identification string sent by the Firefox web browser when making requests on the internet.
     * @returns a random firefox user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.firefoxUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (Macintosh; U; PPC Mac OS X 10_9_1 rv:5.0) Gecko/1979-07-30 Firefox/37.0"
     * ```
     */
    firefoxUserAgent(): string;

    /**
     * The name given to a person at birth.
     * @returns a random first name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.firstName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Josiah"
     * ```
     */
    firstName(): string;

    /**
     * Data type representing floating-point numbers with 32 bits of precision in computing.
     * @returns a random float32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.float32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1.9168120387159532e+38
     * ```
     */
    float32(): number;

    /**
     * Float32 value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random float32 range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.float32Range(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 4.126601219177246
     * ```
     */
    float32Range(min: number, max: number): number;

    /**
     * Data type representing floating-point numbers with 64 bits of precision in computing.
     * @returns a random float64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.float64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1.012641406418422e+308
     * ```
     */
    float64(): number;

    /**
     * Float64 value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random float64 range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.float64Range(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 4.126600960731799
     * ```
     */
    float64Range(min: number, max: number): number;

    /**
     * Edible plant part, typically sweet, enjoyed as a natural snack or dessert.
     * @returns a random fruit
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.fruit())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Avocado"
     * ```
     */
    fruit(): string;

    /**
     * Date that has occurred after the current moment in time.
     * @returns a random futuretime
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.futureTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2024-12-18T19:55:55.767585665+01:00"
     * ```
     */
    futureTime(): string;

    /**
     * Communication failure in the high-performance, open-source universal RPC framework.
     * @returns a random grpc error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.gRPCError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    gRPCError(): string;

    /**
     * User-selected online username or alias used for identification in games.
     * @returns a random gamertag
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.gamertag())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "BraveArmadillo"
     * ```
     */
    gamertag(): string;

    /**
     * Classification based on social and cultural norms that identifies an individual.
     * @returns a random gender
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.gender())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "male"
     * ```
     */
    gender(): string;

    /**
     * Abbreviations and acronyms commonly used in the hacking and cybersecurity community.
     * @returns a random hacker abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackerAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "GB"
     * ```
     */
    hackerAbbreviation(): string;

    /**
     * Adjectives describing terms often associated with hackers and cybersecurity experts.
     * @returns a random hacker adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackerAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "auxiliary"
     * ```
     */
    hackerAdjective(): string;

    /**
     * Noun representing an element, tool, or concept within the realm of hacking and cybersecurity.
     * @returns a random hacker noun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackerNoun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "application"
     * ```
     */
    hackerNoun(): string;

    /**
     * Informal jargon and slang used in the hacking and cybersecurity community.
     * @returns a random hacker phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackerPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Try to transpile the EXE sensor, maybe it will deconstruct the wireless interface!"
     * ```
     */
    hackerPhrase(): string;

    /**
     * Verbs associated with actions and activities in the field of hacking and cybersecurity.
     * @returns a random hacker verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackerVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "read"
     * ```
     */
    hackerVerb(): string;

    /**
     * Verb describing actions and activities related to hacking, often involving computer systems and security.
     * @returns a random hackering verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hackeringVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quantifying"
     * ```
     */
    hackeringVerb(): string;

    /**
     * Auxiliary verb that helps the main verb complete the sentence.
     * @returns a random helping verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.helpingVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "are"
     * ```
     */
    helpingVerb(): string;

    /**
     * Six-digit code representing a color in the color model.
     * @returns a random hex color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "#bd38ac"
     * ```
     */
    hexColor(): string;

    /**
     * Hexadecimal representation of an 128-bit unsigned integer.
     * @returns a random hexuint128
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint128())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691402ee58a2330f9c5"
     * ```
     */
    hexUint128(): string;

    /**
     * Hexadecimal representation of an 16-bit unsigned integer.
     * @returns a random hexuint16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b"
     * ```
     */
    hexUint16(): string;

    /**
     * Hexadecimal representation of an 256-bit unsigned integer.
     * @returns a random hexuint256
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint256())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691402ee58a2330f9c54b727953d2379f94d23ea4cdad195b6a"
     * ```
     */
    hexUint256(): string;

    /**
     * Hexadecimal representation of an 32-bit unsigned integer.
     * @returns a random hexuint32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c90"
     * ```
     */
    hexUint32(): string;

    /**
     * Hexadecimal representation of an 64-bit unsigned integer.
     * @returns a random hexuint64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa1b0c903d687691"
     * ```
     */
    hexUint64(): string;

    /**
     * Hexadecimal representation of an 8-bit unsigned integer.
     * @returns a random hexuint8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hexUint8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "0xaa"
     * ```
     */
    hexUint8(): string;

    /**
     * Paragraph showcasing the use of trendy and unconventional vocabulary associated with hipster culture.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random hipster paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hipsterParagraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Offal forage pinterest direct trade pug. Skateboard food truck flannel cold-pressed church-key.<br />Keffiyeh wolf pop-up jean shorts before they sold out. Hoodie roof portland intelligentsia gastropub."
     * ```
     */
    hipsterParagraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * Sentence showcasing the use of trendy and unconventional vocabulary associated with hipster culture.
     * @param wordcount - Word Count
     * @returns a random hipster sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hipsterSentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Offal forage pinterest direct trade pug."
     * ```
     */
    hipsterSentence(wordcount: number): string;

    /**
     * Trendy and unconventional vocabulary used by hipsters to express unique cultural preferences.
     * @returns a random hipster word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hipsterWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "offal"
     * ```
     */
    hipsterWord(): string;

    /**
     * An activity pursued for leisure and pleasure.
     * @returns a random hobby
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hobby())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Candy making"
     * ```
     */
    hobby(): string;

    /**
     * Unit of time equal to 60 minutes.
     * @returns a random hour
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.hour())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 21
     * ```
     */
    hour(): number;

    /**
     * Failure or issue occurring within a client software that sends requests to web servers.
     * @returns a random http client error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpClientError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpClientError(): string;

    /**
     * A problem with a web http request.
     * @returns a random http error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpError(): string;

    /**
     * Verb used in HTTP requests to specify the desired action to be performed on a resource.
     * @returns a random http method
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpMethod())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "HEAD"
     * ```
     */
    httpMethod(): string;

    /**
     * Failure or issue occurring within a server software that recieves requests from clients.
     * @returns a random http server error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpServerError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    httpServerError(): string;

    /**
     * Random http status code.
     * @returns a random http status code
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpStatusCode())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 400
     * ```
     */
    httpStatusCode(): number;

    /**
     * Three-digit number returned by a web server to indicate the outcome of an HTTP request.
     * @returns a random http status code simple
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpStatusCodeSimple())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 200
     * ```
     */
    httpStatusCodeSimple(): number;

    /**
     * Number indicating the version of the HTTP protocol used for communication between a client and a server.
     * @returns a random http version
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.httpVersion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "HTTP/1.0"
     * ```
     */
    httpVersion(): string;

    /**
     * Web address pointing to an image file that can be accessed and displayed online.
     * @param width - Width
     * @param height - Height
     * @returns a random image url
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.imageUrl(500,500))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "https://picsum.photos/500/500"
     * ```
     */
    imageUrl(width: number, height: number): string;

    /**
     * Adjective describing a non-specific noun.
     * @returns a random indefinite adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.indefiniteAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "somebody"
     * ```
     */
    indefiniteAdjective(): string;

    /**
     * Attribute used to define the name of an input element in web forms.
     * @returns a random input name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.inputName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "last_name"
     * ```
     */
    inputName(): string;

    /**
     * Signed 16-bit integer, capable of representing values from 32,768 to 32,767.
     * @returns a random int16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.int16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -4595
     * ```
     */
    int16(): number;

    /**
     * Signed 32-bit integer, capable of representing values from -2,147,483,648 to 2,147,483,647.
     * @returns a random int32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.int32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -15831539
     * ```
     */
    int32(): number;

    /**
     * Signed 64-bit integer, capable of representing values from -9,223,372,036,854,775,808 to -9,223,372,036,854,775,807.
     * @returns a random int64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.int64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 5195529898953699000
     * ```
     */
    int64(): number;

    /**
     * Signed 8-bit integer, capable of representing values from -128 to 127.
     * @returns a random int8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.int8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -115
     * ```
     */
    int8(): number;

    /**
     * Integer value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random intrange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.intRange(3,5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 3
     * ```
     */
    intRange(min: number, max: number): number;

    /**
     * Word expressing emotion.
     * @returns a random interjection
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.interjection())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wow"
     * ```
     */
    interjection(): string;

    /**
     * Adjective used to ask questions.
     * @returns a random interrogative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.interrogativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "what"
     * ```
     */
    interrogativeAdjective(): string;

    /**
     * Verb that does not require a direct object to complete its meaning.
     * @returns a random intransitive verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.intransitiveVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "skip"
     * ```
     */
    intransitiveVerb(): string;

    /**
     * Numerical label assigned to devices on a network for identification and communication.
     * @returns a random ipv4 address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.ipv4Address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "234.106.177.171"
     * ```
     */
    ipv4Address(): string;

    /**
     * Numerical label assigned to devices on a network, providing a larger address space than IPv4 for internet communication.
     * @returns a random ipv6 address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.ipv6Address())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "3aea:ef6a:38b1:7cab:7f0:946c:a3a9:cb90"
     * ```
     */
    ipv6Address(): string;

    /**
     * International standard code for uniquely identifying securities worldwide.
     * @returns a random isin
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.isin())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "FOS4BL2MVY60"
     * ```
     */
    isin(): string;

    /**
     * Position or role in employment, involving specific tasks and responsibilities.
     * @returns a random job
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.job())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Company":"Xatori","Title":"Representative","Descriptor":"Future","Level":"Tactics"}
     * ```
     */
    job(): Record<string, string>;

    /**
     * Word used to describe the duties, requirements, and nature of a job.
     * @returns a random job descriptor
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.jobDescriptor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Internal"
     * ```
     */
    jobDescriptor(): string;

    /**
     * Random job level.
     * @returns a random job level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.jobLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Identity"
     * ```
     */
    jobLevel(): string;

    /**
     * Specific title for a position or role within a company or organization.
     * @returns a random job title
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.jobTitle())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Representative"
     * ```
     */
    jobTitle(): string;

    /**
     * System of communication using symbols, words, and grammar to convey meaning between individuals.
     * @returns a random language
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.language())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Esperanto"
     * ```
     */
    language(): string;

    /**
     * Shortened form of a language's name.
     * @returns a random language abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.languageAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "eo"
     * ```
     */
    languageAbbreviation(): string;

    /**
     * Set of guidelines and standards for identifying and representing languages in computing and internet protocols.
     * @returns a random language bcp
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.languageBcp())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "he-IL"
     * ```
     */
    languageBcp(): string;

    /**
     * The family name or surname of an individual.
     * @returns a random last name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.lastName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Abshire"
     * ```
     */
    lastName(): string;

    /**
     * Geographic coordinate specifying north-south position on Earth's surface.
     * @returns a random latitude
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.latitude())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 11.394086
     * ```
     */
    latitude(): number;

    /**
     * Latitude number between the given range (default min=0, max=90).
     * @param min - Min
     * @param max - Max
     * @returns a random latitude range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.latitudeRange(0,90))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 50.697043
     * ```
     */
    latitudeRange(min: number, max: number): number;

    /**
     * Character or symbol from the American Standard Code for Information Interchange (ASCII) character set.
     * @returns a random letter
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.letter())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "W"
     * ```
     */
    letter(): string;

    /**
     * ASCII string with length N.
     * @param count - Count
     * @returns a random lettern
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.letterN(3))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "WCp"
     * ```
     */
    letterN(count: number): string;

    /**
     * Replace ? with random generated letters.
     * @param str - String
     * @returns a random lexify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.lexify("none"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    lexify(str: string): string;

    /**
     * Verb that Connects the subject of a sentence to a subject complement.
     * @returns a random linking verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.linkingVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "had"
     * ```
     */
    linkingVerb(): string;

    /**
     * Classification used in logging to indicate the severity or priority of a log entry.
     * @returns a random log level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.logLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "error"
     * ```
     */
    logLevel(): string;

    /**
     * Geographic coordinate indicating east-west position on Earth's surface.
     * @returns a random longitude
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.longitude())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 22.788172
     * ```
     */
    longitude(): number;

    /**
     * Longitude number between the given range (default min=0, max=180).
     * @param min - Min
     * @param max - Max
     * @returns a random longitude range
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.longitudeRange(0,180))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 101.394086
     * ```
     */
    longitudeRange(min: number, max: number): number;

    /**
     * Paragraph of the Lorem Ipsum placeholder text used in design and publishing.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random lorem ipsum paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.loremIpsumParagraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Accusamus et voluptatum voluptatem nisi. Nostrum atque molestias reprehenderit alias.<br />Reiciendis ut eos ut ad. Ea magni recusandae id fuga."
     * ```
     */
    loremIpsumParagraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * Sentence of the Lorem Ipsum placeholder text used in design and publishing.
     * @param wordcount - Word Count
     * @returns a random lorem ipsum sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.loremIpsumSentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Accusamus et voluptatum voluptatem nisi."
     * ```
     */
    loremIpsumSentence(wordcount: number): string;

    /**
     * Word of the Lorem Ipsum placeholder text used in design and publishing.
     * @returns a random lorem ipsum word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.loremIpsumWord())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "accusamus"
     * ```
     */
    loremIpsumWord(): string;

    /**
     * Midday meal, often lighter than dinner, eaten around noon.
     * @returns a random lunch
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.lunch())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tortellini skewers"
     * ```
     */
    lunch(): string;

    /**
     * Unique identifier assigned to network interfaces, often used in Ethernet networks.
     * @returns a random mac address
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.macAddress())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "87:2d:cd:bc:0d:f3"
     * ```
     */
    macAddress(): string;

    /**
     * Name between a person's first name and last name.
     * @returns a random middle name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.middleName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sage"
     * ```
     */
    middleName(): string;

    /**
     * Non-hostile creatures in Minecraft, often used for resources and farming.
     * @returns a random minecraft animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "chicken"
     * ```
     */
    minecraftAnimal(): string;

    /**
     * Component of an armor set in Minecraft, such as a helmet, chestplate, leggings, or boots.
     * @returns a random minecraft armor part
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftArmorPart())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "leggings"
     * ```
     */
    minecraftArmorPart(): string;

    /**
     * Classification system for armor sets in Minecraft, indicating their effectiveness and protection level.
     * @returns a random minecraft armor tier
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftArmorTier())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "leather"
     * ```
     */
    minecraftArmorTier(): string;

    /**
     * Distinctive environmental regions in the game, characterized by unique terrain, vegetation, and weather.
     * @returns a random minecraft biome
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftBiome())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "plain"
     * ```
     */
    minecraftBiome(): string;

    /**
     * Items used to change the color of various in-game objects.
     * @returns a random minecraft dye
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftDye())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "purple"
     * ```
     */
    minecraftDye(): string;

    /**
     * Consumable items in Minecraft that provide nourishment to the player character.
     * @returns a random minecraft food
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftFood())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pufferfish"
     * ```
     */
    minecraftFood(): string;

    /**
     * Powerful hostile creature in the game, often found in challenging dungeons or structures.
     * @returns a random minecraft mob boss
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftMobBoss())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ender dragon"
     * ```
     */
    minecraftMobBoss(): string;

    /**
     * Aggressive creatures in the game that actively attack players when encountered.
     * @returns a random minecraft mob hostile
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftMobHostile())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "blaze"
     * ```
     */
    minecraftMobHostile(): string;

    /**
     * Creature in the game that only becomes hostile if provoked, typically defending itself when attacked.
     * @returns a random minecraft mob neutral
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftMobNeutral())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "dolphin"
     * ```
     */
    minecraftMobNeutral(): string;

    /**
     * Non-aggressive creatures in the game that do not attack players.
     * @returns a random minecraft mob passive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftMobPassive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "axolotl"
     * ```
     */
    minecraftMobPassive(): string;

    /**
     * Naturally occurring minerals found in the game Minecraft, used for crafting purposes.
     * @returns a random minecraft ore
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftOre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "iron"
     * ```
     */
    minecraftOre(): string;

    /**
     * Items in Minecraft designed for specific tasks, including mining, digging, and building.
     * @returns a random minecraft tool
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftTool())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "pickaxe"
     * ```
     */
    minecraftTool(): string;

    /**
     * The profession or occupation assigned to a villager character in the game.
     * @returns a random minecraft villager job
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftVillagerJob())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "carpenter"
     * ```
     */
    minecraftVillagerJob(): string;

    /**
     * Measure of a villager's experience and proficiency in their assigned job or profession.
     * @returns a random minecraft villager level
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftVillagerLevel())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "novice"
     * ```
     */
    minecraftVillagerLevel(): string;

    /**
     * Designated area or structure in Minecraft where villagers perform their job-related tasks and trading.
     * @returns a random minecraft villager station
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftVillagerStation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "lectern"
     * ```
     */
    minecraftVillagerStation(): string;

    /**
     * Tools and items used in Minecraft for combat and defeating hostile mobs.
     * @returns a random minecraft weapon
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftWeapon())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "sword"
     * ```
     */
    minecraftWeapon(): string;

    /**
     * Atmospheric conditions in the game that include rain, thunderstorms, and clear skies, affecting gameplay and ambiance.
     * @returns a random minecraft weather
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftWeather())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "clear"
     * ```
     */
    minecraftWeather(): string;

    /**
     * Natural resource in Minecraft, used for crafting various items and building structures.
     * @returns a random minecraft wood
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minecraftWood())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "oak"
     * ```
     */
    minecraftWood(): string;

    /**
     * Unit of time equal to 60 seconds.
     * @returns a random minute
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.minute())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 9
     * ```
     */
    minute(): number;

    /**
     * Division of the year, typically 30 or 31 days long.
     * @returns a random month
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.month())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 10
     * ```
     */
    month(): string;

    /**
     * String Representation of a month name.
     * @returns a random month string
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.monthString())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "October"
     * ```
     */
    monthString(): string;

    /**
     * A story told through moving pictures and sound.
     * @returns a random movie
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.movie())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Name":"Sherlock Jr.","Genre":"Music"}
     * ```
     */
    movie(): Record<string, string>;

    /**
     * Category that classifies movies based on common themes, styles, and storytelling approaches.
     * @returns a random movie genre
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.movieGenre())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Film-Noir"
     * ```
     */
    movieGenre(): string;

    /**
     * Title or name of a specific film used for identification and reference.
     * @returns a random movie name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.movieName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sherlock Jr."
     * ```
     */
    movieName(): string;

    /**
     * The given and family name of an individual.
     * @returns a random name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.name())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Josiah Thiel"
     * ```
     */
    name(): string;

    /**
     * A title or honorific added before a person's name.
     * @returns a random name prefix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.namePrefix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mr."
     * ```
     */
    namePrefix(): string;

    /**
     * A title or designation added after a person's name.
     * @returns a random name suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nameSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sr."
     * ```
     */
    nameSuffix(): string;

    /**
     * Unit of time equal to One billionth (10^-9) of a second.
     * @returns a random nanosecond
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nanosecond())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 953698829
     * ```
     */
    nanosecond(): number;

    /**
     * Attractive and appealing combinations of colors, returns an list of color hex codes.
     * @returns a random nice colors
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.niceColors())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "MediumVioletRed"
     * ```
     */
    niceColors(): string[];

    /**
     * Person, place, thing, or idea, named or referred to in a sentence.
     * @returns a random noun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.noun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "hand"
     * ```
     */
    noun(): string;

    /**
     * Ideas, qualities, or states that cannot be perceived with the five senses.
     * @returns a random noun abstract
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounAbstract())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "philosophy"
     * ```
     */
    nounAbstract(): string;

    /**
     * Group of animals, like a 'pack' of wolves or a 'flock' of birds.
     * @returns a random noun collective animal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounCollectiveAnimal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "school"
     * ```
     */
    nounCollectiveAnimal(): string;

    /**
     * Group of people or things regarded as a unit.
     * @returns a random noun collective people
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounCollectivePeople())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "bevy"
     * ```
     */
    nounCollectivePeople(): string;

    /**
     * Group of objects or items, such as a 'bundle' of sticks or a 'cluster' of grapes.
     * @returns a random noun collective thing
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounCollectiveThing())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "wad"
     * ```
     */
    nounCollectiveThing(): string;

    /**
     * General name for people, places, or things, not specific or unique.
     * @returns a random noun common
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounCommon())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "company"
     * ```
     */
    nounCommon(): string;

    /**
     * Names for physical entities experienced through senses like sight, touch, smell, or taste.
     * @returns a random noun concrete
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounConcrete())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "train"
     * ```
     */
    nounConcrete(): string;

    /**
     * Items that can be counted individually.
     * @returns a random noun countable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounCountable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "weekend"
     * ```
     */
    nounCountable(): string;

    /**
     * Word that introduces a noun and identifies it as a noun.
     * @returns a random noun determiner
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounDeterminer())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "this"
     * ```
     */
    nounDeterminer(): string;

    /**
     * Phrase with a noun as its head, functions within sentence like a noun.
     * @returns a random noun phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "a brave fuel"
     * ```
     */
    nounPhrase(): string;

    /**
     * Specific name for a particular person, place, or organization.
     * @returns a random noun proper
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounProper())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Rowan Atkinson"
     * ```
     */
    nounProper(): string;

    /**
     * Items that can't be counted individually.
     * @returns a random noun uncountable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.nounUncountable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "butter"
     * ```
     */
    nounUncountable(): string;

    /**
     * Mathematical concept used for counting, measuring, and expressing quantities or values.
     * @param min - Min
     * @param max - Max
     * @returns a random number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.number(-2147483648,2147483647))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * -15831539
     * ```
     */
    number(min: number, max: number): number;

    /**
     * Replace # with random numerical values.
     * @param str - String
     * @returns a random numerify
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.numerify("none"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    numerify(str: string): string;

    /**
     * The specific identification string sent by the Opera web browser when making requests on the internet.
     * @returns a random opera user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.operaUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Opera/10.45 (X11; Linux i686; en-US) Presto/2.13.288 Version/13.00"
     * ```
     */
    operaUserAgent(): string;

    /**
     * Distinct section of writing covering a single theme, composed of multiple sentences.
     * @param paragraphcount - Paragraph Count
     * @param sentencecount - Sentence Count
     * @param wordcount - Word Count
     * @param paragraphseparator - Paragraph Separator
     * @returns a random paragraph
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.paragraph(2,2,5,"\u003cbr /\u003e"))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Quickly up brace lung anyway. Then bravo mirror hundreds his.<br />Party nobody person anything wit. She from above Chinese those."
     * ```
     */
    paragraph(paragraphcount: number, sentencecount: number, wordcount: number, paragraphseparator: string): string;

    /**
     * Secret word or phrase used to authenticate access to a system or account.
     * @param lower - Lower
     * @param upper - Upper
     * @param numeric - Numeric
     * @param special - Special
     * @param space - Space
     * @param length - Length
     * @returns a random password
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.password(true,false,true,true,false,12))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "z42x8h!47-9r"
     * ```
     */
    password(lower: boolean, upper: boolean, numeric: boolean, special: boolean, space: boolean, length: number): string;

    /**
     * Date that has occurred before the current moment in time.
     * @returns a random pasttime
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pastTime())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "2024-12-17T23:55:55.77424777+01:00"
     * ```
     */
    pastTime(): string;

    /**
     * Personal data, like name and contact details, used for identification and communication.
     * @returns a random person
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.person())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"FirstName":"Josiah","LastName":"Thiel","Gender":"male","SSN":"558821916","Image":"https://picsum.photos/367/273","Hobby":"Winemaking","Job":{"Company":"Headlight","Title":"Administrator","Descriptor":"Chief","Level":"Configuration"},"Address":{"Address":"6992 Inletstad, Las Vegas, Rhode Island 82271","Street":"6992 Inletstad","City":"Las Vegas","State":"Rhode Island","Zip":"82271","Country":"Sweden","Latitude":-75.921372,"Longitude":109.436476},"Contact":{"Phone":"4361943393","Email":"janisbarrows@hessel.net"},"CreditCard":{"Type":"Discover","Number":"4525298222125328","Exp":"01/29","Cvv":"282"}}
     * ```
     */
    person(): Record<string, unknown>;

    /**
     * Affectionate nickname given to a pet.
     * @returns a random pet name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.petName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Nacho"
     * ```
     */
    petName(): string;

    /**
     * Numerical sequence used to contact individuals via telephone or mobile devices.
     * @returns a random phone
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.phone())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "7053883851"
     * ```
     */
    phone(): string;

    /**
     * Formatted phone number of a person.
     * @returns a random phone formatted
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.phoneFormatted())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "1-053-883-8516"
     * ```
     */
    phoneFormatted(): string;

    /**
     * A small group of words standing together.
     * @returns a random phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.phrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "many thanks"
     * ```
     */
    phrase(): string;

    /**
     * Adjective indicating ownership or possession.
     * @returns a random possessive adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.possessiveAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "his"
     * ```
     */
    possessiveAdjective(): string;

    /**
     * Words used to express the relationship of a noun or pronoun to other words in a sentence.
     * @returns a random preposition
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.preposition())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "out"
     * ```
     */
    preposition(): string;

    /**
     * Preposition that can be formed by combining two or more prepositions.
     * @returns a random preposition compound
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.prepositionCompound())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "apart from"
     * ```
     */
    prepositionCompound(): string;

    /**
     * Two-word combination preposition, indicating a complex relation.
     * @returns a random preposition double
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.prepositionDouble())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "outside of"
     * ```
     */
    prepositionDouble(): string;

    /**
     * Phrase starting with a preposition, showing relation between elements in a sentence..
     * @returns a random preposition phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.prepositionPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "of a fuel"
     * ```
     */
    prepositionPhrase(): string;

    /**
     * Single-word preposition showing relationships between 2 parts of a sentence.
     * @returns a random preposition simple
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.prepositionSimple())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "of"
     * ```
     */
    prepositionSimple(): string;

    /**
     * The amount of money or value assigned to a product, service, or asset in a transaction.
     * @param min - Min
     * @param max - Max
     * @returns a random price
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.price(0,1000))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 563.3
     * ```
     */
    price(min: number, max: number): number;

    /**
     * An item created for sale or use.
     * @returns a random product
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.product())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"Name":"Quartz Teal Scale","Description":"Bravo mirror hundreds his party nobody. Anything wit she from above Chinese those choir toilet as you of other enormously.","Categories":["mobile phones","food and groceries","furniture"],"Price":82.9,"Features":["durable"],"Color":"green","Material":"bronze","UPC":"084020104876"}
     * ```
     */
    product(): Record<string, unknown>;

    /**
     * Classification grouping similar products based on shared characteristics or functions.
     * @returns a random product category
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productCategory())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "mobile phones"
     * ```
     */
    productCategory(): string;

    /**
     * Explanation detailing the features and characteristics of a product.
     * @returns a random product description
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productDescription())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Up brace lung anyway then bravo mirror hundreds his party. Person anything wit she from above Chinese those choir toilet as you."
     * ```
     */
    productDescription(): string;

    /**
     * Specific characteristic of a product that distinguishes it from others products.
     * @returns a random product feature
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productFeature())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "touchscreen"
     * ```
     */
    productFeature(): string;

    /**
     * The substance from which a product is made, influencing its appearance, durability, and properties.
     * @returns a random product material
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productMaterial())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "alloy"
     * ```
     */
    productMaterial(): string;

    /**
     * Distinctive title or label assigned to a product for identification and marketing.
     * @returns a random product name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Stream Gold Robot"
     * ```
     */
    productName(): string;

    /**
     * Standardized barcode used for product identification and tracking in retail and commerce.
     * @returns a random product upc
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.productUpc())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "092964558555"
     * ```
     */
    productUpc(): string;

    /**
     * Formal system of instructions used to create software and perform computational tasks.
     * @returns a random programming language
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.programmingLanguage())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Ceylon"
     * ```
     */
    programmingLanguage(): string;

    /**
     * Word used in place of a noun to avoid repetition.
     * @returns a random pronoun
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronoun())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    pronoun(): string;

    /**
     * Pronoun that points out specific people or things.
     * @returns a random pronoun demonstrative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounDemonstrative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "these"
     * ```
     */
    pronounDemonstrative(): string;

    /**
     * Pronoun that does not refer to a specific person or thing.
     * @returns a random pronoun indefinite
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounIndefinite())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "anyone"
     * ```
     */
    pronounIndefinite(): string;

    /**
     * Pronoun used to ask questions.
     * @returns a random pronoun interrogative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounInterrogative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "who"
     * ```
     */
    pronounInterrogative(): string;

    /**
     * Pronoun used as the object of a verb or preposition.
     * @returns a random pronoun object
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounObject())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "you"
     * ```
     */
    pronounObject(): string;

    /**
     * Pronoun referring to a specific persons or things.
     * @returns a random pronoun personal
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounPersonal())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "you"
     * ```
     */
    pronounPersonal(): string;

    /**
     * Pronoun indicating ownership or belonging.
     * @returns a random pronoun possessive
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounPossessive())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "mine"
     * ```
     */
    pronounPossessive(): string;

    /**
     * Pronoun referring back to the subject of the sentence.
     * @returns a random pronoun reflective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounReflective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "herself"
     * ```
     */
    pronounReflective(): string;

    /**
     * Pronoun that introduces a clause, referring back to a noun or pronoun.
     * @returns a random pronoun relative
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.pronounRelative())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "that"
     * ```
     */
    pronounRelative(): string;

    /**
     * Adjective derived from a proper noun, often used to describe nationality or origin.
     * @returns a random proper adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.properAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Confucian"
     * ```
     */
    properAdjective(): string;

    /**
     * Adjective that indicates the quantity or amount of something.
     * @returns a random quantitative adjective
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.quantitativeAdjective())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "several"
     * ```
     */
    quantitativeAdjective(): string;

    /**
     * Statement formulated to inquire or seek clarification.
     * @returns a random question
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.question())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Forage pinterest direct trade pug skateboard food truck flannel cold-pressed?"
     * ```
     */
    question(): string;

    /**
     * Direct repetition of someone else's words.
     * @returns a random quote
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.quote())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "\"Forage pinterest direct trade pug skateboard food truck flannel cold-pressed.\" - Lukas Ledner"
     * ```
     */
    quote(): string;

    /**
     * Randomly selected value from a slice of int.
     * @param ints - Integers
     * @returns a random random int
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.randomInt([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 14
     * ```
     */
    randomInt(ints: number[]): number;

    /**
     * Return a random string from a string array.
     * @param strs - Strings
     * @returns a random random string
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.randomString(["none","how","these","keep","trip","congolese","choir","computer","still","far"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "none"
     * ```
     */
    randomString(strs: string[]): string[];

    /**
     * Randomly selected value from a slice of uint.
     * @param uints - Unsigned Integers
     * @returns a random random uint
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.randomUint([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 14
     * ```
     */
    randomUint(uints: number[]): number;

    /**
     * Color defined by red, green, and blue light values.
     * @returns a random rgb color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.rgbColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [13,150,143]
     * ```
     */
    rgbColor(): number[];

    /**
     * Malfunction occuring during program execution, often causing abrupt termination or unexpected behavior.
     * @returns a random runtime error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.runtimeError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    runtimeError(): string;

    /**
     * The specific identification string sent by the Safari web browser when making requests on the internet.
     * @returns a random safari user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.safariUserAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (iPhone; CPU iPhone OS 7_3_2 like Mac OS X; en-US) AppleWebKit/534.34.8 (KHTML, like Gecko) Version/3.0.5 Mobile/8B114 Safari/6534.34.8"
     * ```
     */
    safariUserAgent(): string;

    /**
     * Colors displayed consistently on different web browsers and devices.
     * @returns a random safe color
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.safeColor())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "black"
     * ```
     */
    safeColor(): string;

    /**
     * An institution for formal education and learning.
     * @returns a random school
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.school())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Valley View Private Middle School"
     * ```
     */
    school(): string;

    /**
     * Unit of time equal to 1/60th of a minute.
     * @returns a random second
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.second())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 9
     * ```
     */
    second(): number;

    /**
     * Set of words expressing a statement, question, exclamation, or command.
     * @param wordcount - Word Count
     * @returns a random sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.sentence(5))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Quickly up brace lung anyway."
     * ```
     */
    sentence(wordcount: number): string;

    /**
     * Shuffles an array of ints.
     * @param ints - Integers
     * @returns a random shuffle ints
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.shuffleInts([14,8,13]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * [8,13,14]
     * ```
     */
    shuffleInts(ints: number[]): number[];

    /**
     * Shuffle an array of strings.
     * @param strs - Strings
     * @returns a random shuffle strings
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.shuffleStrings(["none","how","these","keep","trip","congolese","choir","computer","still","far"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * ["these","congolese","far","choir","still","trip","computer","how","keep","none"]
     * ```
     */
    shuffleStrings(strs: string[]): string[];

    /**
     * Group of words that expresses a complete thought.
     * @returns a random simple sentence
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.simpleSentence())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "A brave fuel enormously beautifully stack easy day less badly in a bunch."
     * ```
     */
    simpleSentence(): string;

    /**
     * Catchphrase or motto used by a company to represent its brand or values.
     * @returns a random slogan
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.slogan())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Pride. De-engineered!"
     * ```
     */
    slogan(): string;

    /**
     * Random snack.
     * @returns a random snack
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.snack())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Hoisin marinated wing pieces"
     * ```
     */
    snack(): string;

    /**
     * Unique nine-digit identifier used for government and financial purposes in the United States.
     * @returns a random ssn
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.ssn())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "853698829"
     * ```
     */
    ssn(): string;

    /**
     * Governmental division within a country, often having its own laws and government.
     * @returns a random state
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.state())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Massachusetts"
     * ```
     */
    state(): string;

    /**
     * Shortened 2-letter form of a country's state.
     * @returns a random state abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.stateAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "AA"
     * ```
     */
    stateAbbreviation(): string;

    /**
     * Public road in a city or town, typically with houses and buildings on each side.
     * @returns a random street
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.street())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "53883 Villageborough"
     * ```
     */
    street(): string;

    /**
     * Name given to a specific road or street.
     * @returns a random street name
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.streetName())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Fall"
     * ```
     */
    streetName(): string;

    /**
     * Numerical identifier assigned to a street.
     * @returns a random street number
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.streetNumber())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25388"
     * ```
     */
    streetNumber(): string;

    /**
     * Directional or descriptive term preceding a street name, like 'East' or 'Main'.
     * @returns a random street prefix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.streetPrefix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "West"
     * ```
     */
    streetPrefix(): string;

    /**
     * Designation at the end of a street name indicating type, like 'Avenue' or 'Street'.
     * @returns a random street suffix
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.streetSuffix())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ville"
     * ```
     */
    streetSuffix(): string;

    /**
     * Randomly split people into teams.
     * @param people - Strings
     * @param teams - Strings
     * @returns a random teams
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.teams(["none","how","these","keep","trip","congolese","choir","computer","still","far"],["unless","army","party","riches","theirs","instead","here","mine","whichever","that"]))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {"instead":["trip"],"whichever":["keep"],"that":["none"],"army":["congolese"],"riches":["choir"],"theirs":["still"],"mine":["how"],"unless":["these"],"party":["far"],"here":["computer"]}
     * ```
     */
    teams(people: string[], teams: string[]): Record<string, Array<string>>;

    /**
     * Region where the same standard time is used, based on longitudinal divisions of the Earth.
     * @returns a random timezone
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.timezone())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Tonga Standard Time"
     * ```
     */
    timezone(): string;

    /**
     * Abbreviated 3-letter word of a timezone.
     * @returns a random timezone abbreviation
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.timezoneAbbreviation())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "TST"
     * ```
     */
    timezoneAbbreviation(): string;

    /**
     * Full name of a timezone.
     * @returns a random timezone full
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.timezoneFull())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "(UTC+13:00) Nuku'alofa"
     * ```
     */
    timezoneFull(): string;

    /**
     * The difference in hours from Coordinated Universal Time (UTC) for a specific region.
     * @returns a random timezone offset
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.timezoneOffset())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 13
     * ```
     */
    timezoneOffset(): number;

    /**
     * Geographic area sharing the same standard time.
     * @returns a random timezone region
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.timezoneRegion())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Asia/Manila"
     * ```
     */
    timezoneRegion(): string;

    /**
     * Verb that requires a direct object to complete its meaning.
     * @returns a random transitive verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.transitiveVerb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "bother"
     * ```
     */
    transitiveVerb(): string;

    /**
     * Unsigned 16-bit integer, capable of representing values from 0 to 65,535.
     * @returns a random uint16
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uint16())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 15082
     * ```
     */
    uint16(): number;

    /**
     * Unsigned 32-bit integer, capable of representing values from 0 to 4,294,967,295.
     * @returns a random uint32
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uint32())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 2131652109
     * ```
     */
    uint32(): number;

    /**
     * Unsigned 64-bit integer, capable of representing values from 0 to 18,446,744,073,709,551,615.
     * @returns a random uint64
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uint64())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 5195529898953699000
     * ```
     */
    uint64(): number;

    /**
     * Unsigned 8-bit integer, capable of representing values from 0 to 255.
     * @returns a random uint8
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uint8())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 234
     * ```
     */
    uint8(): number;

    /**
     * Non-negative integer value between given range.
     * @param min - Min
     * @param max - Max
     * @returns a random uintrange
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uintRange(0,4294967295))
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 2131652109
     * ```
     */
    uintRange(min: number, max: number): number;

    /**
     * Web address that specifies the location of a resource on the internet.
     * @returns a random url
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.url())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "http://www.forwardtransition.biz/enhance/benchmark"
     * ```
     */
    url(): string;

    /**
     * String sent by a web browser to identify itself when requesting web content.
     * @returns a random user agent
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.userAgent())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Mozilla/5.0 (X11; Linux i686) AppleWebKit/5311 (KHTML, like Gecko) Chrome/37.0.834.0 Mobile Safari/5311"
     * ```
     */
    userAgent(): string;

    /**
     * Unique identifier assigned to a user for accessing an account or system.
     * @returns a random username
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.username())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Abshire5538"
     * ```
     */
    username(): string;

    /**
     * 128-bit identifier used to uniquely identify objects or entities in computer systems.
     * @returns a random uuid
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.uuid())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "ea6ab1ab-f06c-4990-835d-e628b7e659e1"
     * ```
     */
    uuid(): string;

    /**
     * Occurs when input data fails to meet required criteria or format specifications.
     * @returns a random validation error
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.validationError())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * {}
     * ```
     */
    validationError(): string;

    /**
     * Edible plant or part of a plant, often used in savory cooking or salads.
     * @returns a random vegetable
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.vegetable())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Broccoli"
     * ```
     */
    vegetable(): string;

    /**
     * Word expressing an action, event or state.
     * @returns a random verb
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.verb())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "dig"
     * ```
     */
    verb(): string;

    /**
     * Phrase that Consists of a verb and its modifiers, expressing an action or state.
     * @returns a random verb phrase
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.verbPhrase())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "cheerfully cry enormously beautifully with easy day less badly"
     * ```
     */
    verbPhrase(): string;

    /**
     * Day of the week excluding the weekend.
     * @returns a random weekday
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.weekday())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "Sunday"
     * ```
     */
    weekday(): string;

    /**
     * Basic unit of language representing a concept or thing, consisting of letters and having meaning.
     * @returns a random word
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.word())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "quickly"
     * ```
     */
    word(): string;

    /**
     * Period of 365 days, the time Earth takes to orbit the Sun.
     * @returns a random year
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.year())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * 1979
     * ```
     */
    year(): number;

    /**
     * Numerical code for postal address sorting, specific to a geographic area.
     * @returns a random zip
     * @example
     * ```ts
     *import { Faker } from "k6/x/faker"
     *
     *let faker = new Faker(11)
     *
     *export default function () {
     *  console.log(faker.zen.zip())
     *}
     *
     *```
     * **Output** (formatted as JSON value)
     *```json
     * "25388"
     * ```
     */
    zip(): string;
  }

}
