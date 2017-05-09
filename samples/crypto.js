import crypto from 'k6/crypto';

export default function() {
    // Shorthand API
    let hash = crypto.sha1("some text", "hex");
    console.log(hash);

    // Flexible API
    let hasher = crypto.createHash("sha1")
    hasher.update("some other text")
    console.log(hasher.digest("hex"))
    console.log(hasher.digest("base64"))
}
