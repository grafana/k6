export default async function () {
  const aliceKeyPair = await importKeys(
    alicePublicKeyData,
    alicePrivateKeyData
  );

  console.log("alice: ", JSON.stringify(aliceKeyPair));
}

const importKeys = async (publicKeyData, privateKeyData) => {
  const publicKey = await crypto.subtle.importKey(
    "raw",
    publicKeyData,
    { name: "ECDSA", namedCurve: "P-256" },
    true,
    ["verify"]
  );

  const privateKey = await crypto.subtle.importKey(
    "pkcs8",
    privateKeyData,
    { name: "ECDSA", namedCurve: "P-256" },
    true,
    ["sign"]
  );

  return { publicKey: publicKey, privateKey: privateKey };
};

const alicePublicKeyData = new Uint8Array([
  4, 106, 149, 34, 76, 184, 103, 101, 35, 234, 57, 76, 231, 21, 188, 244, 15,
  179, 101, 113, 24, 6, 17, 21, 195, 60, 181, 73, 154, 170, 206, 21, 244, 102,
  50, 21, 235, 66, 107, 55, 97, 177, 160, 21, 167, 210, 15, 233, 76, 31, 135,
  131, 215, 123, 149, 171, 153, 231, 152, 197, 87, 176, 32, 39, 137,
]);

const alicePrivateKeyData = new Uint8Array([
  48, 129, 135, 2, 1, 0, 48, 19, 6, 7, 42, 134, 72, 206, 61, 2, 1, 6, 8, 42,
  134, 72, 206, 61, 3, 1, 7, 4, 109, 48, 107, 2, 1, 1, 4, 32, 41, 167, 202, 58,
  174, 179, 236, 224, 240, 214, 91, 12, 207, 12, 10, 4, 200, 252, 81, 163, 175,
  76, 120, 60, 102, 201, 132, 40, 177, 74, 244, 226, 161, 68, 3, 66, 0, 4, 106,
  149, 34, 76, 184, 103, 101, 35, 234, 57, 76, 231, 21, 188, 244, 15, 179, 101,
  113, 24, 6, 17, 21, 195, 60, 181, 73, 154, 170, 206, 21, 244, 102, 50, 21,
  235, 66, 107, 55, 97, 177, 160, 21, 167, 210, 15, 233, 76, 31, 135, 131, 215,
  123, 149, 171, 153, 231, 152, 197, 87, 176, 32, 39, 137,
]);
