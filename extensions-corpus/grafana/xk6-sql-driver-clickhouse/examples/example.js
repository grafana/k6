import sql from "k6/x/sql";
import driver from "k6/x/sql/driver/clickhouse";

const db = sql.open(driver, "clickhouse://127.0.0.1:19000");

export function setup() {
  db.exec(`CREATE TABLE IF NOT EXISTS hits_by_user_url
  (
      UserID UInt32,
      URL String,
      EventTime DateTime
  )
  ENGINE = MergeTree
  PRIMARY KEY (UserID, URL)
  ORDER BY (UserID, URL, EventTime)
  SETTINGS index_granularity = 8192, index_granularity_bytes = 0;`);
}

export function teardown() {
  db.close();
}

export default function () {
  db.exec(`INSERT INTO hits_by_user_url 
    (UserID, URL, EventTime)
    SELECT * FROM generateRandom('UserID UInt32, URL String, EventTime DateTime')
    LIMIT 100;`);
}
