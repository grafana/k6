export default async function () {
  const generatedKey = await crypto.subtle.generateKey(
    {
      name: "HMAC",
      hash: { name: "SHA-1" },
    },
    true,
    ["sign", "verify"]
  );

  const encoded = string2ArrayBuffer("Hello World");

  // Signs the encoded data with the provided key using the HMAC algorithm
  // the returned signature can be verified using the verify method.
  const signature = await crypto.subtle.sign("HMAC", generatedKey, encoded);

  // Verifies the signature of the encoded data with the provided key using the HMAC algorithm
  const verified = await crypto.subtle.verify(
    "HMAC",
    generatedKey,
    signature,
    encoded
  );

  console.log("verified: ", verified);
}

function string2ArrayBuffer(str) {
  var buf = new ArrayBuffer(str.length * 2); // 2 bytes for each char
  var bufView = new Uint16Array(buf);
  for (var i = 0, strLen = str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
}
