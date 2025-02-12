import { b64decode } from "k6/encoding";

export default async function () {
  // transmitted data is the base64 of the initialization vector + encrypted data
  // that unusually transmitted over the network
  const transmittedData = base64Decode(
    "whzEN310mrlWIH/icf0dMquRZ2ENyfOzkvPuu92WR/9F8dbeFM8EGUVNIhaS"
  );

  // keyData is the key used to decrypt the data, that is usually stored in a secure location
  // for the purpose of this example, we are using a static key
  const keyData = new Uint8Array([
    109, 151, 76, 33, 232, 253, 176, 90, 94, 40, 146, 227, 139, 208, 245, 139,
    69, 215, 55, 197, 43, 122, 160, 178, 228, 104, 4, 115, 138, 159, 119, 49,
  ]);

  const result = await decrypt(keyData, transmittedData);

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
    "raw",
    keyData,
    { name: "AES-GCM", length: "256" },
    true,
    ["decrypt"]
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
