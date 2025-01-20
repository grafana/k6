import { crypto } from "k6/experimental/webcrypto";

export default async function () {
  const key = await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: 1024, // Can be 1024, 2048, or 4096
      publicExponent: new Uint8Array([1, 0, 1]), // 24-bit representation of 65537
      hash: { name: "SHA-256" }, // Could be "SHA-1", "SHA-256", "SHA-384", or "SHA-512"
    },
    true,
    ["sign", "verify"] // Key usages
  );

  console.log(JSON.stringify(key));
}
