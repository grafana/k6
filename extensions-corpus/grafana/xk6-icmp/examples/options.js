import { ping } from "k6/x/icmp"

export default function () {
  const host = "8.8.8.8"

  console.log(`Pinging ${host}:`);

  const result = ping(host, {
    timeout: 2000,
    count: 3
  });

  if (result) {
    console.log(`Host ${host} is reachable`);
  } else {
    console.error(`Host ${host} is unreachable`);
  }
}
