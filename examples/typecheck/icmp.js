import { IP,  ping,  pingAsync  } from "k6/x/icmp";

export default async function () {
  const reachable = await pingAsync(
    "127.0.0.1",
    {
      count: 1,
      preferred_ip_version: IP.V4,
      timeout: "1s",
    },
    (error, detail) => {
      if (error || detail === undefined) {
        return;
      }
      console.log(`${detail.target_ip}: ${detail.alive ? "reachable" : "unreachable"}`);
    },
  );

  console.log(`localhost is ${reachable ? "reachable" : "unreachable"}`);
}
