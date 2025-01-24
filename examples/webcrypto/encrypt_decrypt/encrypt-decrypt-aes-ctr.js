export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "AES-CTR",
      length: 256,
    },
    true,
    ["encrypt", "decrypt"]
  );

  const encoded = string2ArrayBuffer("Hello World");
  const counter = crypto.getRandomValues(new Uint8Array(16));

  const ciphertext = await crypto.subtle.encrypt(
    {
      name: "AES-CTR",
      counter,
      length: 64,
    },
    key,
    encoded
  );

  const plaintext = await crypto.subtle.decrypt(
    {
      name: "AES-CTR",
      counter,
      length: 64,
    },
    key,
    ciphertext
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

function string2ArrayBuffer(str) {
  var buf = new ArrayBuffer(str.length * 2); // 2 bytes for each char
  var bufView = new Uint16Array(buf);
  for (var i = 0, strLen = str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
}
