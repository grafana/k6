# xk6-sql-driver-mysql

Database driver extension for [xk6-sql](https://github.com/grafana/xk6-sql) k6 extension to support MySQL database.

## Example

```JavaScript file=examples/example.js
import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/mysql";

// The second argument is a MySQL connection string, e.g.
// myuser:mypass@tcp(127.0.0.1:3306)/mydb
const db = sql.open(driver, "");

export function setup() {
  db.exec(`
    CREATE TABLE IF NOT EXISTS roster
      (
        id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
        given_name VARCHAR(50) NOT NULL,
        family_name VARCHAR(50) NULL
      );
  `);
}

export function teardown() {
  db.close();
}

export default function () {
  let result = db.exec(`
    INSERT INTO roster
      (given_name, family_name)
    VALUES
      ('Peter', 'Pan'),
      ('Wendy', 'Darling'),
      ('Tinker', 'Bell'),
      ('James', 'Hook');
  `);
  console.log(`${result.rowsAffected()} rows inserted`);

  let rows = db.query("SELECT * FROM roster WHERE given_name = ?;", "Peter");
  for (const row of rows) {
    // Convert array of ASCII integers into strings. See https://github.com/grafana/xk6-sql/issues/12
    console.log(`${String.fromCharCode(...row.family_name)}, ${String.fromCharCode(...row.given_name)}`);
  }
}
```

## Usage

Check the [xk6-sql documentation](https://github.com/grafana/xk6-sql) on how to use this database driver.


### TLS Support

> [!IMPORTANT]
>
> TLS support has been adopted more or less unchanged from xk6-sql v0.4.1.
> Refactoring is required.

To enable TLS support, call `loadTLS` from the script, before calling `open`. [examples/example-tls.js](examples/example-tls.js) is an example.

`loadTLS` accepts the following options:

```javascript
loadTLS({
  enableTLS: true,
  insecureSkipTLSverify: true,
  minVersion: TLS_1_2,
  // Possible values: TLS_1_0, TLS_1_1, TLS_1_2, TLS_1_3
  caCertFile: '/filepath/to/ca.pem',
  clientCertFile: '/filepath/to/client-cert.pem',
  clientKeyFile: '/filepath/to/client-key.pem',
});
```
