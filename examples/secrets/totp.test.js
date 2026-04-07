// TOTP generation with secrets API
//
// Run with:
//   k6 run --secret-source=mock=totp_secret="GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" examples/secrets/totp.test.js
//
// Note:
// Don't use the mock secret source in production. This source is designed for testing purposes only.

import secrets from 'k6/secrets';
import { TOTP } from 'https://jslib.k6.io/totp/1.0.0/index.js';
import { check } from 'k6';

export default async function () {
    // Get TOTP secret from secrets API
    const secret = await secrets.get('totp_secret');

    // Create TOTP instance with 6-digit codes
    const totp = new TOTP(secret, 6);

    // Generate current TOTP code
    const code = await totp.gen();
    console.log(`Generated TOTP code: ${code}`);

    // Verify the code (should be valid since we just generated it)
    const isValid = await totp.verify(code);

    check(isValid, {
        'TOTP code is valid': (v) => v === true,
    });

    console.log(`Code verification: ${isValid ? 'PASSED' : 'FAILED'}`);
}
