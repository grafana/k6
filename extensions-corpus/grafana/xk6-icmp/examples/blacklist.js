import { ping } from "k6/x/icmp"

export const options = {
  blacklistIPs: ['8.8.8.8/24'],
}

export default function () {
  const host = "8.8.8.8"

  console.log(`Pinging ${host}:`);

  try {
    ping(host)
  } catch (e) {
    console.error(`Error pinging ${host}: ${e.message}`);
  }
}
