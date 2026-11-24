export default async function () {
    const publicJwk = {
        "kty":"OKP",
        "crv":"X25519",
        "x":"6A_qFvvKXsog9CcjJSOv6KLP_KfEyarubOhQCRfVj2g",
        "key_ops":["deriveKey","deriveBits"],
        "ext":true
    }

    const privateJwk = {
        "kty":"OKP",
        "crv":"X25519",
        "x":"6A_qFvvKXsog9CcjJSOv6KLP_KfEyarubOhQCRfVj2g",
        "d":"F8yaoTzBYLBBCxjJ_IEBtaqk5PVXsMaBoCZCrBtsUwo",
        "key_ops":["deriveKey","deriveBits"],
        "ext":true
    }

    const publicKey = await crypto.subtle.importKey(
        "jwk",
        publicJwk,
        { name: "X25519" },
        true,
        []
    );

    const privateKey = await crypto.subtle.importKey(
        "jwk",
        privateJwk,
        { name: "X25519" },
        true,
        ["deriveKey", "deriveBits"]
    );

    console.log("public key: " + JSON.stringify(publicKey));
    console.log("private key: " + JSON.stringify(privateKey));

    const exportedPublicJwk = await crypto.subtle.exportKey("jwk", publicKey);
    console.log("exported public jwk: " + JSON.stringify(exportedPublicJwk));

    const exportedPrivateJwk = await crypto.subtle.exportKey("jwk", privateKey);
    console.log("exported private jwk: " + JSON.stringify(exportedPrivateJwk));
}