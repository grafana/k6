import {
  generateTOTP,
  verifyTOTP,
} from "https://jsr.io/@rabbit-company/totp/1.0.1/src/totp.ts";

const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ";

export default async function () {
  const code = await generateTOTP(secret, { digits: 6 });
  const valid = await verifyTOTP(code, secret);

  if (!/^\d{6}$/.test(code)) {
    throw new Error(`expected a six-digit TOTP code, received ${code}`);
  }
  if (!valid) {
    throw new Error("JSR TOTP did not verify its generated code");
  }
}
