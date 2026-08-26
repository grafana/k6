import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/postgres";

const db = sql.open(driver, "postgres://myuser:mypassword@127.0.0.1:5432/mydb?sslmode=disable");

export function setup() {
  db.exec(`
    CREATE TABLE IF NOT EXISTS roster
      (
        id SERIAL PRIMARY KEY,
        given_name VARCHAR(50) NOT NULL,
        family_name VARCHAR(50) NOT NULL
      );
  `);
}

export function teardown() {
  db.exec("DROP TABLE IF EXISTS roster;");
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

  let rows = db.query("SELECT * FROM roster WHERE given_name = $1;", "Peter");
  for (const row of rows) {
    console.log(`${row.family_name}, ${row.given_name}`);
  }
}
