export default async function () {
    const ed25519KeyPair = await crypto.subtle.generateKey(
        {
          name: "Ed25519",
        },
        true,
        ["sign", "verify"]
    );

    console.log("ed25519 key pair: " + JSON.stringify(ed25519KeyPair));
}