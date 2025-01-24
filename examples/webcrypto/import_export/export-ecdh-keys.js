export default async function () {
  const generatedKeyPair = await crypto.subtle.generateKey(
    {
      name: "ECDH",
      namedCurve: "P-256",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  const exportedPrivateKey = await crypto.subtle.exportKey(
    "pkcs8",
    generatedKeyPair.privateKey
  );
  console.log("exported private key: " + printArrayBuffer(exportedPrivateKey));

  const exportedPublicKey = await crypto.subtle.exportKey(
    "raw",
    generatedKeyPair.publicKey
  );
  console.log("exported public key: " + printArrayBuffer(exportedPublicKey));
}

const printArrayBuffer = (buffer) => {
  let view = new Uint8Array(buffer);
  return Array.from(view);
};
