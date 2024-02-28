import { crypto } from "k6/x/webcrypto";

 export default async function () {
    // const generatedKey = await crypto.subtle.generateKey(
    //     {
    //         name: "ECDH",
    //         namedCurve: "P-256"
    //     },
    //     true,
    //     [
    //         "deriveKey",
    //         "deriveBits"
    //     ]
    // );

    // console.log("generated: " + JSON.stringify(generatedKey));

    const keyData = new Uint8Array([4, 210, 16, 176, 166, 249, 217, 240, 18, 134, 128, 88, 180, 63, 164, 244, 113, 1, 133, 67, 187, 160, 12, 146, 80, 223, 146, 87, 194, 172, 174, 93, 209, 206, 3, 117, 82, 212, 129, 69, 12, 227, 155, 77, 16, 149, 112, 27, 23, 91, 250, 179, 75, 142, 108, 9, 158, 24, 241, 193, 152, 53, 131, 97, 232]);
    console.log("static keyData: " + JSON.stringify(keyData));

    const importedKey = await crypto.subtle.importKey(
        "raw",
        keyData,
        { name: "ECDH", namedCurve: "P-256" },
        true,
        [],
    );

    console.log("imported: " + JSON.stringify(importedKey));

    const exportedKey = await crypto.subtle.exportKey("raw", importedKey);

    console.log("exported: " + JSON.stringify(exportedKey));
}