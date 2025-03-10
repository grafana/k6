export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "X25519",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  console.log(JSON.stringify(key));
}