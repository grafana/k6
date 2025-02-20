import { crypto } from "k6/experimental/webcrypto";

export default async function () {
  const keyPair = await crypto.subtle.generateKey(
    {
      name: "Ed25519",
    },
    true,
    ["sign", "verify"]
  );

  const data = string2ArrayBuffer("Hello World");

  const alg = { name: "Ed25519" };

  // makes a signature of the encoded data with the provided key
  const signature = await crypto.subtle.sign(alg, keyPair.privateKey, data);

  console.log("signature: ", printArrayBuffer(signature));

  //Verifies the signature of the encoded data with the provided key
  const verified = await crypto.subtle.verify(
    alg,
    otherKeyPair.publicKey,
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
