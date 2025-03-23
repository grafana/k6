export default async function () {
    const generatedKeyPair = await crypto.subtle.generateKey(
      {
        name: "X25519",
      },
      true,
      ["deriveKey", "deriveBits"]
    );
  
    const exportedPrivateKey = await crypto.subtle.exportKey(
      "pkcs8",
      generatedKeyPair.privateKey
    );
    console.log("exported private key: " + printArrayBuffer(exportedPrivateKey));
  
    const exportedRawPublicKey = await crypto.subtle.exportKey(
      "raw",
      generatedKeyPair.publicKey
    );
  
    const exportedSpkiPublicKey = await crypto.subtle.exportKey(
      "spki",
      generatedKeyPair.publicKey
    );
  
    console.log("exported public key: " + printArrayBuffer(exportedRawPublicKey));
    console.log("exported spki public key: " + printArrayBuffer(exportedSpkiPublicKey));
  }
  
  const printArrayBuffer = (buffer) => {
    let view = new Uint8Array(buffer);
    return Array.from(view);
  };
  