import { crypto } from "k6/experimental/webcrypto";

export default function () {
  const myUUID = crypto.randomUUID();

  console.log(myUUID);
}
