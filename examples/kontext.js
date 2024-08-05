import { Kontext } from "k6/kontext";

export const options = {
	duration: '30s',
}

const kontext = new Kontext();

export default async function() {
	await kontext.set("foo", "bar");
	const value = await kontext.get("foo");
	console.log(`foo: ${value}`);
}
