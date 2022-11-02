import { sleep } from "k6";
import { crypto } from "k6/x/webcrypto";

console.log(JSON.stringify(crypto));

export default function () {
  const d = crypto.subtle.digest("SHA-256", "Hello, world!").then((hash) => {
    const h = buf2hex(hash);
    console.log(h);
  });
}

function buf2hex(buffer) {
  return [...new Uint8Array(buffer)]
    .map((x) => x.toString(16).padStart(2, "0"))
    .join("");
}
