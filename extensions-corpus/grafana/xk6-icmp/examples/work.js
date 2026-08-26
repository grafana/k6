import { pingAsync } from "k6/x/icmp"

export default async function () {
  //const host = "192.168.1.11"
  //const host = "dns.google.com"
  const host = "192.0.2.1"

  console.log(`Pinging ${host} with callback:`);

  const opts = {
    timeout: 300,
    //count: 5,
  };

  const result = await pingAsync(host, opts, (err, { target, target_ip, sent_at, received_at, seq, ttl, size }) => {
    if (err) {
      console.error(`${target}: ${err}`);

      return
    }

    const rtt = received_at - sent_at;

    console.log(`${size} bytes from ${target} (${target_ip}): icmp_seq=${seq} ttl=${ttl} time=${rtt} ms`);
  });

  if (result) {
    console.log(`Host ${host} is reachable`);
  } else {
    console.error(`Host ${host} is unreachable`);
  }
}
