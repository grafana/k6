import http from 'k6/http';

export let options = {
    tlsCipherSuites: [
        "TLS_RSA_WITH_RC4_128_SHA",
        "TLS_RSA_WITH_AES_128_GCM_SHA256",
    ],
    tlsVersion: {
        min: "ssl3.0",
        max: "tls1.2"
    }
};

export default function() {
  const response = http.get("https://sha256.badssl.com");
};

