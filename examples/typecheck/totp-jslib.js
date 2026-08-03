import { TOTP } from "https://jslib.k6.io/totp/1.0.0/index.js";

const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ";

export default async function () {
  const totp = new TOTP(secret, 6);
  const code = await totp.gen();
  const valid = await totp.verify(code);

  if (!/^\d{6}$/.test(code)) {
    throw new Error(`expected a six-digit TOTP code, received ${code}`);
  }
  if (!valid) {
    throw new Error("jslib TOTP did not verify its generated code");
  }
}
