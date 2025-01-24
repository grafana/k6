export default async function () {
  const generatedKeyPair = await await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: 1024,
      publicExponent: new Uint8Array([1, 0, 1]), // 24-bit representation of 65537
      hash: { name: "SHA-256" },
    },
    true,
    ["sign", "verify"] // Key usages
  );

  const exportedPrivateKey = await crypto.subtle.exportKey(
    "pkcs8",
    generatedKeyPair.privateKey
  );
  console.log("exported private key: " + printArrayBuffer(exportedPrivateKey));

  const exportedPublicKey = await crypto.subtle.exportKey(
    "spki",
    generatedKeyPair.publicKey
  );
  console.log("exported public key: " + printArrayBuffer(exportedPublicKey));
}

const printArrayBuffer = (buffer) => {
  let view = new Uint8Array(buffer);
  return Array.from(view);
};
