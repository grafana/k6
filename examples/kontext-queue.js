import { Kontext } from "k6/kontext";
import { sleep } from "k6";

export const options = {
	scenarios: {
		orderCreation: {
			exec: "orderCreation",
			executor: "shared-iterations",
			vus: 1,
			iterations: 10,
		},

		orderProcessing: {
			exec: "orderProcessing",
			executor: "shared-iterations",
			vus: 2,
			iterations: 10,
		}
	},
}

const kontext = new Kontext();

export async function setup() {
	await kontext.set("order_id", 1);
}

export async function orderCreation() {
	const orderId = await kontext.get("order_id");
	console.log(`[creation] publishing order: ${orderId}`)
	await kontext.lpush("orders", orderId);

	console.log(`[creation] waiting for next user order`)
	await kontext.set("order_id", orderId + 1);
	sleep(getRandomNumber(1, 3));
}

export async function orderProcessing() {
	let order;

	do {
		try {
			order = await kontext.rpop("orders");
		} catch (e) {
			// console.log(`[processing] no order to process`);
			sleep(getRandomNumber(1, 3));
			continue
		}

		if (order == null) {
			sleep(getRandomNumber(1, 3));
			continue
		}

		console.log(`[processing] processing order: ${order}`);
	} while (order < 10);
}

function getRandomNumber(min, max) {
	return Math.floor(Math.random() * (max - min + 1)) + min;
}
