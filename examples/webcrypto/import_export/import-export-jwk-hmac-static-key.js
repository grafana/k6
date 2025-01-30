export default async function () {
  const jwk = {
    alg: "HS256",
    ext: true,
    k: "H6gLp3lw7w27NrPUn00WpcKU-IJojJdNzhL_8F6se2k",
    key_ops: ["sign", "verify"],
    kty: "oct",
  };

  console.log("static key: " + JSON.stringify(jwk));

  const importedKey = await crypto.subtle.importKey(
    "jwk",
    jwk,
    { name: "HMAC", hash: { name: "SHA-256" } },
    true,
    ["sign", "verify"]
  );

  console.log("imported: " + JSON.stringify(importedKey));

  const exportedAgain = await crypto.subtle.exportKey("jwk", importedKey);

  console.log("exported again: " + JSON.stringify(exportedAgain));
}
