import crypto from "k6/crypto";
import encoding from "k6/encoding";
import {sleep} from "k6";

const algToHash = {
    HS256: "sha256",
    HS384: "sha384",
    HS512: "sha512"
};

function sign(data, hashAlg, secret) {
    let hasher = crypto.createHMAC(hashAlg, secret);
    hasher.update(data);

    // Some manual base64 rawurl encoding as `Hasher.digest(encodingType)`
    // doesn't support that encoding type yet.
    return hasher.digest("base64").replace(/\//g, "_").replace(/\+/g, "-").replace(/=/g, "");
}

function encode(payload, secret, algorithm) {
    algorithm = algorithm || "HS256";
    let header = encoding.b64encode(JSON.stringify({ typ: "JWT", alg: algorithm }), "rawurl");
    payload = encoding.b64encode(JSON.stringify(payload), "rawurl", "s");
    let sig = sign(header + "." + payload, algToHash[algorithm], secret);
    return [header, payload, sig].join(".");
}

function decode(token, secret, algorithm) {
    let parts = token.split('.');
    let header = JSON.parse(encoding.b64decode(parts[0], "rawurl", "s"));
    let payload = JSON.parse(encoding.b64decode(parts[1], "rawurl", "s"));
    algorithm = algorithm || algToHash[header.alg];
    if (sign(parts[0] + "." + parts[1], algorithm, secret) != parts[2]) {
        throw Error("JWT signature verification failed");
    }
    return payload;
}

export default function() {
    let message = { key2: "value2" };
    let token = encode(message, "secret");
    console.log("encoded", token);
    let payload = decode(token, "secret");
    console.log("decoded", JSON.stringify(payload));
    sleep(1)
}
