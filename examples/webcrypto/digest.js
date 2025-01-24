export default function () {
  crypto.subtle.digest("SHA-256", stringToArrayBuffer("Hello, world!")).then(
    (hash) => {
      console.log(arrayBufferToHex(hash));
    },
    (err) => {
      throw err;
    }
  );
}

function arrayBufferToHex(buffer) {
  return [...new Uint8Array(buffer)]
    .map((x) => x.toString(16).padStart(2, "0"))
    .join("");
}

function stringToArrayBuffer(s) {
  return Uint8Array.from(new String(s), (x) => x.charCodeAt(0));
}
