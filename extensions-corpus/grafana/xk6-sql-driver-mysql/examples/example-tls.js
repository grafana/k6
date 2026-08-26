// Adapted more or less unchanged from: https://github.com/grafana/xk6-sql/blob/v0.4.1/examples/mysql_secure_test.js
// It will have to be refactored.

import sql from "k6/x/sql";
import { loadTLS, addTLS, TLS_1_2, default as driver } from "k6/x/sql/driver/mysql";

const err = loadTLS({
  enableTLS: true,
  insecureSkipTLSverify: true,
  minVersion: TLS_1_2,
  // Possible values: TLS_1_0, TLS_1_1, TLS_1_2, TLS_1_3
  caCertFile: "ca.pem",
  clientCertFile: "client-cert.pem",
  clientKeyFile: "client-key.pem",
});

const db = sql.open(driver, addTLS("root:password@tcp(localhost:3306)/mysql"));

export function setup() {
  db.exec(`
    CREATE TABLE IF NOT EXISTS keyvalues (
      id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
      \`key\` VARCHAR(50) NOT NULL,
      value VARCHAR(50) NULL
    );
  `);
}

export function teardown() {
  db.close();
}

export default function () {
  db.exec("INSERT INTO keyvalues (`key`, value) VALUES('plugin-name', 'k6-plugin-sql');");

  let results = db.query("SELECT * FROM keyvalues WHERE `key` = ?;", "plugin-name");
  for (const row of results) {
    // Convert array of ASCII integers into strings. See https://github.com/grafana/xk6-sql/issues/12
    console.log(`key: ${String.fromCharCode(...row.key)}, value: ${String.fromCharCode(...row.value)}`);
  }
}
