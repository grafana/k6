import { greet } from "k6/x/types-example";

export default function () {
  console.log(greet("k6"));
}
