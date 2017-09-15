import http from "k6/http";
import { check } from "k6";

export default function() {
  const HOMEPAGE_RESPONSE = http.get("https://www.bbc.co.uk/");
  check(http.get("https://www.bbc.co.uk/"), {
    "status is 200": (r) => r.status == 200,
    "protocol is HTTP/2": (r) => r.proto == "HTTP/2.0",
  });
}
