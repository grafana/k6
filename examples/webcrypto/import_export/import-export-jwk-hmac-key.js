export default async function () {
  const generatedKey = await crypto.subtle.generateKey(
    {
      name: "HMAC",
      hash: { name: "SHA-256" },
    },
    true,
    ["sign", "verify"]
  );

  console.log("generated: " + JSON.stringify(generatedKey));

  const exportedKey = await crypto.subtle.exportKey("jwk", generatedKey);

  console.log("exported: " + JSON.stringify(exportedKey));

  const importedKey = await crypto.subtle.importKey(
    "jwk",
    exportedKey,
    { name: "HMAC", hash: { name: "SHA-256" } },
    true,
    ["sign", "verify"]
  );

  console.log("imported: " + JSON.stringify(importedKey));

  const exportedAgain = await crypto.subtle.exportKey("jwk", importedKey);

  console.log("exported again: " + JSON.stringify(exportedAgain));
}
