import { crypto } from "k6/experimental/webcrypto";

export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "ECDSA",
      namedCurve: "P-256",
    },
    true,
    ["sign", "verify"]
  );

  console.log(JSON.stringify(key));
}
