export default async function () {
    const publicKey = await crypto.subtle.importKey(
        "raw",
        aliceRawPublicKeyData,
        { name: "Ed25519" },
        true,
        ["verify"]
    );

    const privateKey = await crypto.subtle.importKey(
        "pkcs8",
        alicePkcs8PrivateKeyData,
        { name: "Ed25519" },
        true,
        ["sign"]
    );

    const spkiPublicKey = await crypto.subtle.importKey(
        "spki",
        spkiPublicKeyData,
        { name: "Ed25519" },
        true,
        ["verify"]
    );

    console.log("raw public key: ", JSON.stringify(publicKey));
    console.log("pkcs8 private key: ", JSON.stringify(privateKey));
    console.log("spki public key: ", JSON.stringify(spkiPublicKey));
}

const aliceRawPublicKeyData = new Uint8Array([20,143,11,228,219,143,240,246,228,95,189,140,34,196,138,241,105,163,220,110,81,16,167,243,77,251,70,100,130,131,153,43])
const alicePkcs8PrivateKeyData = new Uint8Array([48,46,2,1,0,48,5,6,3,43,101,112,4,34,4,32,235,89,226,177,105,103,230,133,229,2,157,78,107,14,0,197,81,149,209,139,6,37,80,98,219,50,0,38,144,234,156,194])
const spkiPublicKeyData = new Uint8Array([48,42,48,5,6,3,43,101,112,3,33,0,210,238,42,158,126,130,110,253,80,77,38,242,209,88,172,114,11,120,31,243,24,171,47,144,217,186,184,71,152,40,110,168])