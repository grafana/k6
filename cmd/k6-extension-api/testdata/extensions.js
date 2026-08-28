import msgpack from "k6/x/msgpack";
import ssh from "k6/x/ssh";
import tls from "k6/x/tls";
import { Faker } from "k6/x/faker";
import * as dns from "k6/x/dns";
import * as icmp from "k6/x/icmp";
import { Client as RedisClient } from "k6/x/redis";
import { Client as MQTTClient } from "k6/x/mqtt";
import { Socket as TCPSocket } from "k6/x/tcp";
import "k6/x/kubernetes";
import "k6/x/sql";
import "k6/x/sql/driver/azuresql";
import "k6/x/sql/driver/clickhouse";
import "k6/x/sql/driver/mysql";
import "k6/x/sql/driver/postgres";
import "k6/x/sql/driver/sqlserver";

export const options = { vus: 1, iterations: 1 };

export default function () {
	const value = msgpack.unpack(msgpack.pack({ answer: 42 }));
	if (value.answer !== 42) {
		throw new Error("MessagePack extension did not round-trip its value");
	}
	if (typeof ssh.connect !== "function") {
		throw new Error("SSH extension was not registered");
	}
	if (typeof Faker !== "function") {
		throw new Error("Faker extension was not registered");
	}
	if (typeof tls.getCertificate !== "function") {
		throw new Error("TLS extension was not registered");
	}
	if (typeof dns.resolve !== "function" || typeof dns.lookup !== "function") {
		throw new Error("DNS extension was not registered");
	}
	if (typeof icmp.ping !== "function" || typeof icmp.pingAsync !== "function") {
		throw new Error("ICMP extension was not registered");
	}
	if (typeof RedisClient !== "function") {
		throw new Error("Redis extension was not registered");
	}
	new RedisClient("redis://127.0.0.1:6379");
	if (typeof MQTTClient !== "function" || typeof TCPSocket !== "function") {
		throw new Error("MQTT or TCP extension was not registered");
	}
	new MQTTClient();
	new TCPSocket();
}
