export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "HMAC",
      hash: { name: "SHA-512" },
      length: 256,
    },
    true,
    ["sign", "verify"]
  );

  console.log(JSON.stringify(key));
}
