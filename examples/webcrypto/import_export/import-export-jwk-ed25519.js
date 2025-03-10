export default async function () {
    const publicJwk = {
        "kty":"OKP",
        "crv":"Ed25519",
        "x":"o7RbBVJW_6Ua3h5J3MCEGAeXRC6xHvtotIiAadK-xbM",
        "key_ops":["verify"],
        "ext":true
    };

    const privateJwk = { 
        "kty": "OKP", 
        crv: "Ed25519", 
        x: "o7RbBVJW_6Ua3h5J3MCEGAeXRC6xHvtotIiAadK-xbM", 
        d: "lHnUA3j3VmVOCYuF4nzEgbQ9QnaBNXXTLIK45adoyEmjtFsFUlb_pRreHkncwIQYB5dELrEe-2i0iIBp0r7Fsw", 
        key_ops: ["sign"], 
        ext: true 
    }

    const publicKey = await crypto.subtle.importKey(
        "jwk",
        publicJwk,
        { name: "Ed25519" },
        true,
        ["verify"]
    );

    const privateKey = await crypto.subtle.importKey(
        "jwk",
        privateJwk,
        { name: "Ed25519" },
        true,
        ["sign"]
    );

    console.log("public key: " + JSON.stringify(publicKey));
    console.log("private key: " + JSON.stringify(privateKey));

    const exportedPublicJwk = await crypto.subtle.exportKey("jwk", publicKey);
    console.log("exported public jwk: " + JSON.stringify(exportedPublicJwk));

    const exportedPrivateJwk = await crypto.subtle.exportKey("jwk", privateKey);
    console.log("exported private jwk: " + JSON.stringify(exportedPrivateJwk));
}