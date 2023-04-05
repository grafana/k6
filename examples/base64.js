import { check } from "k6";
import encoding from "k6/encoding";

export default function() {
    // Standard base64 encoding/decoding with '=' padding
    let str = "hello world";
    let enc = "aGVsbG8gd29ybGQ=";
    check(null, {
        "is std encoding correct": () => encoding.b64encode(str) === enc,
        "is std decoding correct": () => encoding.b64decode(enc, null, "s") === str
    });

    // Standard base64 encoding/decoding without '=' padding
    str = "hello world";
    enc = "aGVsbG8gd29ybGQ";
    check(null, {
        "is rawstd encoding correct": () => encoding.b64encode(str, 'rawstd') === enc,
        "is rawstd decoding correct": () => encoding.b64decode(enc, 'rawstd', "s") === str
    });

    // URL-safe base64 encoding/decoding with '=' padding
    str = "小飼弾..";
    enc = "5bCP6aO85by-Li4=";
    check(null, {
        "is url encoding correct": () => encoding.b64encode(str, 'url') === enc,
        "is url decoding correct": () => encoding.b64decode(enc, 'url', "s") === str
    });

    // URL-safe base64 encoding/decoding without '=' padding
    str = "小飼弾..";
    enc = "5bCP6aO85by-Li4";
    check(null, {
        "is rawurl encoding correct": () => encoding.b64encode(str, 'rawurl') === enc,
        "is rawurl decoding correct": () => encoding.b64decode(enc, 'rawurl', "s") === str
    });
};
