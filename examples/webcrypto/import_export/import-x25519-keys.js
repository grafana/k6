export default async function () {
    const publicKey = await crypto.subtle.importKey(
        "raw",
        rawPublicKeyData,
        { name: "X25519" },
        true,
        []
    );

    const privateKey = await crypto.subtle.importKey(
        "pkcs8",
        pkcs8PrivateKeyData,
        { name: "X25519" },
        true,
        ["deriveKey", "deriveBits"]
    );

    const spkiPublicKey = await crypto.subtle.importKey(
        "spki",
        spkiPublicKeyData,
        { name: "X25519" },
        true,
        []
    );

    console.log("raw public key: ", JSON.stringify(publicKey));
    console.log("pkcs8 private key: ", JSON.stringify(privateKey));
    console.log("spki public key: ", JSON.stringify(spkiPublicKey));
}

const rawPublicKeyData = new Uint8Array([246,202,188,31,183,153,51,177,149,155,170,93,244,164,24,175,10,174,169,100,246,102,27,26,173,47,156,129,99,185,33,5])
const pkcs8PrivateKeyData = new Uint8Array([48,46,2,1,0,48,5,6,3,43,101,110,4,34,4,32,253,170,153,16,32,119,141,238,33,23,216,192,216,30,27,215,182,34,233,234,137,146,17,5,208,126,153,105,71,154,65,11])
const spkiPublicKeyData = new Uint8Array([48,42,48,5,6,3,43,101,110,3,33,0,246,202,188,31,183,153,51,177,149,155,170,93,244,164,24,175,10,174,169,100,246,102,27,26,173,47,156,129,99,185,33,5])