import faker from "k6/x/faker";

export default function () {
  console.log(faker.person.firstName());
}

// as long as XK6_FAKER_SEED is 11
// output: Josiah
