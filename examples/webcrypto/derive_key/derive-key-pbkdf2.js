function stringToArrayBuffer(str) {
    const buf = new ArrayBuffer(str.length * 2); // 2 bytes for each char
    const bufView = new Uint16Array(buf);
    for (let i = 0, strLen = str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
    }
    return buf;
}

export default async function () {
    // create a low entropy password as array of bytes 
    const password = stringToArrayBuffer("hello world password!")

    // import password as CryptoKey
    const importedKey = await crypto.subtle.importKey('raw', password, 'pbkdf2', false, [
    'deriveBits', 'deriveKey'
    ]);

    // generate random salt
    const saltArray = new Uint8Array(16)
    crypto.getRandomValues(saltArray)
    const salt = saltArray.buffer

    // derive AES-GCM 256 key using  deriveKey 
    const derivedKey = await crypto.subtle.deriveKey(
      {
        name: "PBKDF2",
        salt,
        iterations: 310000,
        hash: "SHA-256"
      },
      importedKey,
      {
        name : "AES-GCM",
        length: 256,
      },
      true,
      ["encrypt", 'decrypt']
    )

    console.log(derivedKey)
}