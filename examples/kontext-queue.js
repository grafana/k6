import { Kontext } from "k6/kontext";

// export const options = {
// 	duration: '30s',
// }

const kontext = new Kontext();

export default async function() {
	await kontext.lpush("mylist", 3)
	await kontext.lpush("mylist", 2)
	await kontext.lpush("mylist", 1)

	let value;
	value = await kontext.rpop("mylist");
	console.log(`popped: ${value}`);
	value = await kontext.rpop("mylist");
	console.log(`popped: ${value}`);
	value = await kontext.rpop("mylist");
	console.log(`popped: ${value}`);
}
