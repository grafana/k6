import msgpack from "k6/x/msgpack";
import ssh from "k6/x/ssh";

export const options = { vus: 1, iterations: 1 };

export default function () {
	console.log(msgpack)
  const value = msgpack.unpack(msgpack.pack({ answer: 42 }));
  if (value.answer !== 42) {
    throw new Error("MessagePack extension did not round-trip its value");
  }
  if (typeof ssh.connect !== "function") {
    throw new Error("SSH extension was not registered");
  }
}
