import faker from "k6/x/faker";

export default function () {
  console.log(faker.person.firstName());
}
