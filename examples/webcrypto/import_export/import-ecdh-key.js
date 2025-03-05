export default async function () {
  const aliceKeyPair = await importKeys(
    alicePublicKeyData,
    alicePrivateKeyData
  );

  const bobKeyPair = await importKeys(bobPublicKeyData, bobPrivateKeyData);

  console.log("alice: ", JSON.stringify(aliceKeyPair));
  console.log("bob: ", JSON.stringify(bobKeyPair));

  // Derive shared secret for Alice
  const aliceSharedSecret = await deriveSharedSecret(
    aliceKeyPair.privateKey,
    bobKeyPair.publicKey
  );

  // Derive shared secret for Bob
  const bobSharedSecret = await deriveSharedSecret(
    bobKeyPair.privateKey,
    aliceKeyPair.publicKey
  );

  console.log("alice shared secret: " + printArrayBuffer(aliceSharedSecret));
  console.log("bob shared secret: " + printArrayBuffer(bobSharedSecret));
}

const importKeys = async (publicKeyData, privateKeyData) => {
  const publicKey = await crypto.subtle.importKey(
    "raw",
    publicKeyData,
    { name: "ECDH", namedCurve: "P-256" },
    true,
    []
  );

  const privateKey = await crypto.subtle.importKey(
    "pkcs8",
    privateKeyData,
    { name: "ECDH", namedCurve: "P-256" },
    true,
    ["deriveKey", "deriveBits"]
  );

  return { publicKey: publicKey, privateKey: privateKey };
};
async function deriveSharedSecret(privateKey, publicKey) {
  return crypto.subtle.deriveBits(
    {
      name: "ECDH",
      public: publicKey, // An ECDH public key from the other party
    },
    privateKey, // Your ECDH private key
    256 // the number of bits to derive
  );
}

const printArrayBuffer = (buffer) => {
  let view = new Uint8Array(buffer);
  return Array.from(view);
};

const alicePublicKeyData = new Uint8Array([
  4, 8, 249, 89, 225, 84, 28, 108, 246, 144, 7, 182, 109, 32, 155, 16, 102, 22,
  66, 253, 148, 220, 48, 6, 106, 21, 123, 98, 229, 191, 20, 200, 35, 5, 208,
  131, 136, 154, 125, 18, 20, 202, 231, 168, 184, 127, 53, 186, 6, 136, 114,
  101, 127, 109, 179, 44, 96, 108, 193, 126, 217, 131, 163, 131, 135,
]);

const alicePrivateKeyData = new Uint8Array([
  48, 129, 135, 2, 1, 0, 48, 19, 6, 7, 42, 134, 72, 206, 61, 2, 1, 6, 8, 42,
  134, 72, 206, 61, 3, 1, 7, 4, 109, 48, 107, 2, 1, 1, 4, 32, 194, 150, 86, 186,
  233, 47, 132, 192, 213, 56, 60, 179, 112, 7, 89, 65, 116, 88, 8, 158, 228,
  172, 190, 234, 143, 152, 33, 175, 47, 0, 39, 79, 161, 68, 3, 66, 0, 4, 8, 249,
  89, 225, 84, 28, 108, 246, 144, 7, 182, 109, 32, 155, 16, 102, 22, 66, 253,
  148, 220, 48, 6, 106, 21, 123, 98, 229, 191, 20, 200, 35, 5, 208, 131, 136,
  154, 125, 18, 20, 202, 231, 168, 184, 127, 53, 186, 6, 136, 114, 101, 127,
  109, 179, 44, 96, 108, 193, 126, 217, 131, 163, 131, 135,
]);

const bobPublicKeyData = new Uint8Array([
  4, 218, 134, 37, 137, 90, 68, 101, 112, 234, 68, 87, 110, 182, 85, 178, 161,
  106, 223, 50, 150, 9, 155, 68, 191, 51, 138, 185, 186, 226, 211, 25, 203, 96,
  193, 213, 68, 7, 181, 238, 52, 154, 113, 56, 76, 86, 44, 245, 128, 194, 103,
  14, 81, 229, 124, 189, 13, 252, 138, 98, 196, 218, 39, 34, 42,
]);

const bobPrivateKeyData = new Uint8Array([
  48, 129, 135, 2, 1, 0, 48, 19, 6, 7, 42, 134, 72, 206, 61, 2, 1, 6, 8, 42,
  134, 72, 206, 61, 3, 1, 7, 4, 109, 48, 107, 2, 1, 1, 4, 32, 59, 168, 213, 160,
  115, 123, 19, 203, 62, 86, 50, 152, 17, 210, 42, 35, 174, 230, 191, 11, 65,
  239, 223, 130, 73, 53, 161, 46, 9, 210, 50, 4, 161, 68, 3, 66, 0, 4, 218, 134,
  37, 137, 90, 68, 101, 112, 234, 68, 87, 110, 182, 85, 178, 161, 106, 223, 50,
  150, 9, 155, 68, 191, 51, 138, 185, 186, 226, 211, 25, 203, 96, 193, 213, 68,
  7, 181, 238, 52, 154, 113, 56, 76, 86, 44, 245, 128, 194, 103, 14, 81, 229,
  124, 189, 13, 252, 138, 98, 196, 218, 39, 34, 42,
]);
