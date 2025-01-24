export default async function () {
  // Generate a key pair for Alice
  const aliceKeyPair = await crypto.subtle.generateKey(
    {
      name: "ECDH",
      namedCurve: "P-256",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  // Generate a key pair for Bob
  const bobKeyPair = await crypto.subtle.generateKey(
    {
      name: "ECDH",
      namedCurve: "P-256",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

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
