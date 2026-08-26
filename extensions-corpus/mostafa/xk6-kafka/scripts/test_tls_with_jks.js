/*
 * This script shows how to load certificates and keys from
 * a JKS keystore, so that they can be used in a configuring TLS.
 * This is just a showcase, and not a complete script.
 * The keystore MUST be created with JKS storetype.
 *
 * ⚠️ The PKCS#12 format is not supported.
 */

import { LoadJKS, TLS_1_2 } from "k6/x/kafka";

// If server and client keystore are separate, then you must
// call LoadJKS twice, once for each keystore.
// This will load the certificates and keys from the keystore
// and write them to the disk, so that they can be used in
// the TLS configuration.
const jks = LoadJKS({
  path: "fixtures/kafka-keystore.jks",
  password: "password",
  clientCertAlias: "localhost",
  clientKeyAlias: "localhost",
  clientKeyPassword: "password",
  serverCaAlias: "caroot",
});
const tlsConfig = {
  enableTls: true,
  insecureSkipTlsVerify: false,
  minVersion: TLS_1_2,

  // The certificates and keys can be loaded from a JKS keystore:
  // clientCertsPem is an array of PEM-encoded certificates, and the filenames
  // will be named "client-cert-0.pem", "client-cert-1.pem", etc.
  // clientKeyPem is the PEM-encoded private key and the filename will be
  // named "client-key.pem".
  // serverCaPem is the PEM-encoded CA certificate and the filename will be
  // named "server-ca.pem".
  clientCertPem: jks["clientCertsPem"][0], // The first certificate in the chain
  clientKeyPem: jks["clientKeyPem"],
  serverCaPem: jks["serverCaPem"],
};

export default function () {
  console.log(Object.keys(jks));
  console.log(jks);
  console.log(tlsConfig);
}
