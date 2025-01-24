import { b64decode } from "k6/encoding";

export default async function () {
  // transmitted data is the base64 of the initialization vector + encrypted data
  // that unusually transmitted over the network
  const transmittedData = base64Decode(
    "drCfxl4O+5FcrHe8Bs0CvKlw3gZpv+S5if3zn7c4BJzHJ35QDFV4sJB0pbDT"
  );

  // keyData is the key used to decrypt the data, that is usually stored in a secure location
  // for the purpose of this example, we are using a static key
  const jwkKeyData = {
    kty: "oct",
    ext: true,
    key_ops: ["decrypt", "encrypt"],
    alg: "A256GCM",
    k: "9Id_8iG6FkGOWmc1S203vGVnTExtpDGxdQN7v7OV9Uc",
  };

  const result = await decrypt(jwkKeyData, transmittedData);

  // should output decrypted message
  // INFO[0000] result: 'my secret message'  source=console
  console.log("result: '" + result + "'");
}

const decrypt = async (keyData, transmittedData) => {
  const initializeVectorLength = 12;

  // the first 12 bytes are the initialization vector
  const iv = new Uint8Array(
    transmittedData.subarray(0, initializeVectorLength)
  );

  // the rest of the transmitted data is the encrypted data
  const encryptedData = new Uint8Array(
    transmittedData.subarray(initializeVectorLength)
  );

  const importedKey = await crypto.subtle.importKey(
    "jwk",
    keyData,
    { name: "AES-GCM", length: 256 },
    true,
    ["encrypt", "decrypt"]
  );

  const plain = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: iv },
    importedKey,
    encryptedData
  );

  return arrayBufferToString(plain);
};

const arrayBufferToString = (buffer) => {
  return String.fromCharCode.apply(null, new Uint8Array(buffer));
};

const base64Decode = (base64String) => {
  return new Uint8Array(b64decode(base64String));
};
