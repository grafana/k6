import {crypto } from "k6/experimental/webcrypto";

export default async function () {
  // Generate a key pair for Alice
  const aliceKeyPair = await crypto.subtle.generateKey(
    {
      name: "X25519",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  // Generate a key pair for Alice
  const bobKeyPair = await crypto.subtle.generateKey(
    {
      name: "X25519",
    },
    true,
    ["deriveKey", "deriveBits"]
  );

  console.log(JSON.stringify(aliceKeyPair));

  console.log(JSON.stringify(bobKeyPair))

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
            name: "X25519",
            public: publicKey // An X25519 public key from the other party
        },
        privateKey, // Your X25519 private key
        256
    )
}

const printArrayBuffer = (buffer) => {
    let view = new Uint8Array(buffer);
    return Array.from(view);
  };
  