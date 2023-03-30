import { crypto } from "k6/x/webcrypto";

 export default async function () {
    try {
        const generatedKey = await crypto.subtle.generateKey(
            {
                name: "HMAC",
                hash: { name: "SHA-256" },
            },
            true,
            [
                "sign",
                "verify",
            ]
        );

        const exportedKey = await crypto.subtle.exportKey("raw", generatedKey);

        const importedKey = await crypto.subtle.importKey(
            "raw",
            exportedKey,
            { name: "HMAC", hash: { name: "SHA-256" } },
            true, ["sign", "verify"]
        );

        console.log(JSON.stringify(importedKey))
    } catch (err) {
        console.log(JSON.stringify(err));
    }

}