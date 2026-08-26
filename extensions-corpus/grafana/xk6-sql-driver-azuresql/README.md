# xk6-sql-driver-azuresql

Database driver extension for [xk6-sql](https://github.com/grafana/xk6-sql) k6 extension to support Microsoft Azure SQL database.

## Example

```JavaScript file=examples/example.js
import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/azuresql";

// The second argument is a MS SQL connection string, e.g.
// Server=127.0.0.1;Database=myDB;fedauth=ActiveDirectoryDefault;
const db = sql.open(driver, "");

export function setup() {
  db.exec(`
    IF object_id('roster') is null
      CREATE TABLE roster
        (
          [id] INT IDENTITY PRIMARY KEY,
          [given_name] varchar(50) NOT NULL,
          [family_name] varchar(50) NOT NULL);
  `);
}

export function teardown() {
  db.close();
}

export default function () {
  let result = db.exec(`
    INSERT INTO roster
      ([given_name], [family_name])
    VALUES
    ('Peter', 'Pan'),
      ('Wendy', 'Darling'),
      ('Tinker', 'Bell'),
      ('James', 'Hook');
  `);
  console.log(`${result.rowsAffected()} rows inserted`);

  let rows = db.query("SELECT * FROM roster WHERE [key] = @p1;", "Peter");
  for (const row of rows) {
    console.log(`${row.family_name}, ${row.given_name}`);
  }
}
```

## Usage

Check the [xk6-sql documentation](https://github.com/grafana/xk6-sql) on how to use this database driver.
