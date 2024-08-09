import { Kontext } from "k6/kontext";
import { sleep } from "k6";
import exec from "k6/execution";

export const options = {
	scenarios: {
		orderCreation: {
			exec: "orderCreation",
			executor: "shared-iterations",
			vus: 1,
			iterations: 10,
			maxDuration: '100s'
		},

		orderProcessing: {
			exec: "orderProcessing",
			executor: "shared-iterations",
			vus: 2,
			iterations: 10,
			maxDuration: '100s'
		},
	},
};

const kontext = new Kontext();

function getRandomNumber(min, max) {
	return Math.floor(Math.random() * (max - min + 1)) + min;
}

export async function setup() {
	// Initialize the order_id to 1
	await kontext.set("order_id", 1);
}

export async function orderCreation() {
	sleep(getRandomNumber(0.3, 1));

	// Simulate producing an order id
	const orderId = await kontext.get("order_id");
	console.log(
		`[vu=${exec.vu.idInTest} scenario=orderCreation] publishing order: ${orderId}`
	);

	// Push the order id to the orders queue to start the processing
	await kontext.lpush("orders", orderId);

	console.log(
		`[vu=${exec.vu.idInTest} scenario=orderCreation] waiting for next user order`
	);
	await kontext.incr("order_id");
}

export async function orderProcessing() {
	let order;

	while (true) {
		try {
			// Pop an order from the orders queue if available
			order = await kontext.rpop("orders");
		} catch (e) {
			// Otherwise, let's wait a bit to check again
			sleep(getRandomNumber(1, 2));
			continue;
		}

		if (order == null) {
			sleep(getRandomNumber(1, 2));
			continue;
		}

		// And order is available, let's break the loop and process it!
		break;
	}

	console.log(
		`[vu=${exec.vu.idInTest} scenario=orderProcessing] processing order: ${order}`
	);

	// Simulate processing the order
	await kontext.decr("order_id");
}
