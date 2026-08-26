import { ping } from "k6/x/icmp"

export default function () {
  const host = "8.8.8.8"

  console.log(`Pinging ${host}:`);

  if (ping(host)) {
    console.log(`Host ${host} is reachable`);
  } else {
    console.error(`Host ${host} is unreachable`);
  }
}
