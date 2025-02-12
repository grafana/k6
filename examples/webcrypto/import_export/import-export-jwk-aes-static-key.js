export default async function () {
  const jwk = {
    alg: "A256CBC",
    ext: true,
    k: "LhR2VJFb1NJ8HORgOn7LNKLXhUqPsTjC65UAWFb4GKI",
    key_ops: ["encrypt", "decrypt"],
    kty: "oct",
  };

  console.log("static key: " + JSON.stringify(jwk));

  const importedKey = await crypto.subtle.importKey(
    "jwk",
    jwk,
    { name: "AES-CBC", length: 256 },
    true,
    ["encrypt", "decrypt"]
  );

  console.log("imported: " + JSON.stringify(importedKey));

  const exportedAgain = await crypto.subtle.exportKey("jwk", importedKey);

  console.log("exported again: " + JSON.stringify(exportedAgain));
}
