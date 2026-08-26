[![API Reference](https://img.shields.io/badge/API-reference-blue?logo=readme&logoColor=lightgray)](https://sql.x.k6.io)
[![GitHub Release](https://img.shields.io/github/v/release/grafana/xk6-sql)](https://github.com/grafana/xk6-sql/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/xk6-sql)](https://goreportcard.com/report/github.com/grafana/xk6-sql)
[![GitHub Actions](https://github.com/grafana/xk6-sql/actions/workflows/validate.yml/badge.svg)](https://github.com/grafana/xk6-sql/actions/workflows/validate.yml)

# xk6-sql

**Use SQL databases from k6 tests.**

xk6-sql is a [Grafana k6 extension](https://grafana.com/docs/k6/latest/extensions/) that enables the use of SQL databases in [k6](https://grafana.com/docs/k6/latest/) tests.

Check out the API documentation [here](https://sql.x.k6.io). The TypeScript declaration file can be downloaded from [here](https://sql.x.k6.io/index.d.ts).

To use the TypeScript declaration file in your IDE (e.g. Visual Studio Code), you need to create a `jsconfig.json` (or `tsconfig.json`) file with the following content:

```json file=examples/jsconfig.json
{
  "compilerOptions": {
    "target": "ES6",
    "module": "ES6",
    "paths": {
      "k6/x/sql": ["./typings/xk6-sql/index.d.ts"]
    }
  }
}
```

You will need to update the TypeScript declaration file location in the example above to where you downloaded it.

## Usage

To use the xk6-sql API, the `k6/x/sql` module and the driver module corresponding to the database type should be imported. In the example below, `k6/x/sql/driver/ramsql` is the RamSQL database driver module.

The driver module exports a driver ID. This driver identifier should be used to identify the database driver to be used in the API functions.

**example**

```javascript file=examples/example.js
import sql from "k6/x/sql";

// the actual database driver should be used instead of ramsql
import driver from "k6/x/sql/driver/ramsql";

const db = sql.open(driver, "roster_db");

export function setup() {
  db.exec(`
    CREATE TABLE IF NOT EXISTS roster
      (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        given_name VARCHAR NOT NULL,
        family_name VARCHAR NOT NULL
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

  let rows = db.query("SELECT * FROM roster WHERE given_name = $1;", "Peter");
  for (const row of rows) {
    console.log(`${row.family_name}, ${row.given_name}`);
  }
}
```

## Build

The [xk6](https://github.com/grafana/xk6) build tool can be used to build a k6 that will include **xk6-sql** extension and database drivers.

> [!IMPORTANT]
> In the command line bellow, **xk6-sql-driver-ramsql** is just an example, it should be replaced with the database driver extension you want to use.
> For example use `--with github.com/grafana/xk6-sql-driver-mysql` to access MySQL databases.

```bash
xk6 build --with github.com/grafana/xk6-sql@latest --with github.com/grafana/xk6-sql-driver-ramsql
```

For more build options and how to use xk6, check out the [xk6 documentation](https://github.com/grafana/xk6).

Supported RDBMSs includes: `mysql`, `postgres`, `sqlite3`, `sqlserver`, `azuresql`, `clickhouse`.

Check the [xk6-sql-driver GitHub topic](https://github.com/topics/xk6-sql-driver) to discover database driver extensions.

## Drivers

To use the xk6-sql extension, one or more database driver extensions should also be embedded. Database driver extension names typically start with the prefix `xk6-sql-driver-` followed by the name of the database, for example `xk6-sql-driver-mysql` is the name of the MySQL database driver extension.

For easier discovery, the `xk6-sql-driver` topic is included in the database driver extensions repository. The [xk6-sql-driver GitHub topic search](https://github.com/topics/xk6-sql-driver) therefore lists the available driver extensions.

### Create driver

Check the [grafana/xk6-sql-driver-ramsql](https://github.com/grafana/xk6-sql-driver-ramsql) template repository to create a new driver extension. This is a working driver extension with instructions in its README for customization.

[Postgres driver extension](https://github.com/grafana/xk6-sql-driver-postgres) and [MySQL driver extension](https://github.com/grafana/xk6-sql-driver-mysql) are also good examples.

## Feedback

If you find the **xk6-sql** extension useful, please star the repo. The number of stars will affect the time allocated for maintenance.

[![Stargazers over time](https://starchart.cc/grafana/xk6-sql.svg?variant=adaptive)](https://starchart.cc/grafana/xk6-sql)

## Contribute

If you want to contribute or help with the development of **xk6-sql**, start by reading [CONTRIBUTING.md](CONTRIBUTING.md). 
