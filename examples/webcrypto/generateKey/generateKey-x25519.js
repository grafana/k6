export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "ECDH",
      namedCurve: "X25519",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  console.log(JSON.stringify(key));
}