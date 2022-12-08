import { crypto } from "k6/x/webcrypto";

export default function () {
  const myUUID = crypto.randomUUID();

  console.log(myUUID);
}
