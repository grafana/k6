import { crypto } from "k6/x/webcrypto";

 export default async function () {
    const generatedKey = await crypto.subtle.generateKey(
        {
            name: "ECDH",
            namedCurve: "P-256"
        },
        true,
        [
            "deriveKey",
            "deriveBits"
        ]
    );

    console.log("generated: " + JSON.stringify(generatedKey));

    // const exportedKey = await crypto.subtle.exportKey("raw", generatedKey);
    // console.log("exported: " + exportedKey);
}