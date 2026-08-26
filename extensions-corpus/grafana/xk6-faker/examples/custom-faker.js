import { Faker } from "k6/x/faker";

const faker = new Faker(11);

export default function () {
  console.log(faker.person.firstName());
}

// output: Josiah
