import redis from "k6/x/redis";
import exec from "k6/execution";

export const options = {
    vus: 10,
    iterations: 200,
    insecureSkipTLSVerify: true,
};

// Instantiate a new Redis client using a URL
// const client = new redis.Client('rediss://localhost:6379')
const client = new redis.Client({
    password: "tjkbZ8jrwz3pGiku",
    socket:{
        host: "localhost",
        port: 6379,
        tls: {
            ca: [open('docker/tests/tls/ca.crt')],
            cert: open('docker/tests/tls/client.crt'),  // client cert
            key: open('docker/tests/tls/client.key'),  // client private key
        }
    }
});

export default async function () {
    // VUs are executed in a parallel fashion,
    // thus, to ensure that parallel VUs are not
    // modifying the same key at the same time,
    // we use keys indexed by the VU id.
    const key = `foo-${exec.vu.idInTest}`;

    await client.set(`foo-${exec.vu.idInTest}`, 1)

    let value = await client.get(`foo-${exec.vu.idInTest}`)
    value = await client.incrBy(`foo-${exec.vu.idInTest}`, value)
    
    await client.del(`foo-${exec.vu.idInTest}`)
    const exists = await client.exists(`foo-${exec.vu.idInTest}`)
    if (exists !== 0) {
        throw new Error("foo should have been deleted");
    }
}
