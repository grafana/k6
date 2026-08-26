import tls from "k6/x/tls";
import { check } from "k6";

export const options = {
  iterations: 1,
};

export default async function () {
  let res = await tls.getCertificate("quickpizza.grafana.com");

  // Below a test case when the certificate is expired:
  // let res = await tls.getCertificate("expired-rsa-dv.ssl.com");

  console.log(`
    subj: ${res.subject.common_name}
    issuer: ${res.issuer.common_name}
    issued on: ${res.issued}
    expires at ${res.expires}
    fingerprint: ${res.fingerprint}`);

  check(res, {
    "tls certificate is not expired": (c) => c.expires > Date.now(),
  });
}
