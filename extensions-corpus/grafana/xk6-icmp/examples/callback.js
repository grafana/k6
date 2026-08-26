import { pingAsync } from "k6/x/icmp"

export default async function () {
  const host = "8.8.8.8"

  console.log(`Pinging ${host} with callback:`);

  const opts = {
    timeout: 3000,
    count: 5
  };

  const result = await pingAsync(host, opts, ({ target, sent_at, received_at, seq, ttl, size, options }, error) => {
    if (error) {
      console.error(`${target}: ${error}`);

      return
    }

    const rtt = received_at - sent_at;

    console.log(`${size} bytes from ${target}: icmp_seq=${seq} ttl=${ttl} time=${rtt} ms`);
  });

  if (result) {
    console.log(`Host ${host} is reachable`);
  } else {
    console.error(`Host ${host} is unreachable`);
  }
}
