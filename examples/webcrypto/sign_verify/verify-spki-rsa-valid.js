export default async function () {
  const keyPair = await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: 1024, // Can be 1024, 2048, or 4096
      publicExponent: new Uint8Array([1, 0, 1]), // 24-bit representation of 65537
      hash: { name: "SHA-256" }, // Could be "SHA-1", "SHA-256", "SHA-384", or "SHA-512"
    },
    true,
    ["sign", "verify"]
  );

  console.log("keyPair: ", JSON.stringify(keyPair));

  const data = string2ArrayBuffer("Hello World");

  const alg = { name: "RSASSA-PKCS1-v1_5", hash: { name: "SHA-256" } };

  console.log("private key type: " + keyPair.privateKey.type);
  console.log("public key type: " + keyPair.publicKey.type);

  // makes a signature of the encoded data with the provided key
  const signature = await crypto.subtle.sign(alg, keyPair.privateKey, data);

  console.log("signature: ", printArrayBuffer(signature));

  //Verifies the signature of the encoded data with the provided key
  const verified = await crypto.subtle.verify(
    alg,
    keyPair.publicKey,
    signature,
    data
  );

  console.log("verified: ", verified);
}

const string2ArrayBuffer = (str) => {
  let buf = new ArrayBuffer(str.length * 2); // 2 bytes for each char
  let bufView = new Uint16Array(buf);
  for (let i = 0, strLen = str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
};

const printArrayBuffer = (buffer) => {
  let view = new Uint8Array(buffer);
  return Array.from(view);
};
