export default async function () {
  const generatedKey = await crypto.subtle.generateKey(
    {
      name: "HMAC",
      hash: { name: "SHA-256" },
    },
    true,
    ["sign", "verify"]
  );

  const exportedKey = await crypto.subtle.exportKey("raw", generatedKey);

  const importedKey = await crypto.subtle.importKey(
    "raw",
    exportedKey,
    { name: "HMAC", hash: { name: "SHA-256" } },
    true,
    ["sign", "verify"]
  );

  console.log(JSON.stringify(importedKey));
}
