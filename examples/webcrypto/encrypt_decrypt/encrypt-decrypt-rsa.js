export default async function () {
  const keyPair = await crypto.subtle.generateKey(
    {
      name: "RSA-OAEP",
      modulusLength: 2048,
      publicExponent: new Uint8Array([1, 0, 1]),
      hash: { name: "SHA-1" },
    },
    true,
    ["encrypt", "decrypt"]
  );

  const encoded = stringToArrayBuffer("Hello, World!");

  const cipherText = await crypto.subtle.encrypt(
    {
      name: "RSA-OAEP",
    },
    keyPair.publicKey,
    encoded
  );

  // ciphertext.byteLength * 8, vector.privateKey.algorithm.modulusLength
  console.log("cipherText's byteLength: ", cipherText.byteLength * 8);
  console.log(
    "algorithm's modulusLength: ",
    keyPair.privateKey.algorithm.modulusLength
  );

  const plaintext = await crypto.subtle.decrypt(
    {
      name: "RSA-OAEP",
    },
    keyPair.privateKey,
    cipherText
  );

  console.log(
    "deciphered text == original text: ",
    arrayBufferToHex(plaintext) === arrayBufferToHex(encoded)
  );
}

function arrayBufferToHex(buffer) {
  return [...new Uint8Array(buffer)]
    .map((x) => x.toString(16).padStart(2, "0"))
    .join("");
}

function stringToArrayBuffer(str) {
  var buf = new ArrayBuffer(str.length * 2); // 2 bytes for each char
  var bufView = new Uint16Array(buf);
  for (var i = 0, strLen = str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
}
