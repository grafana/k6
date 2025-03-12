// k6 run --secret-source=file=file.secret secrets.test.js
import secrets from "k6/secrets";

export default async () => {
	const my_secret = await secrets.get("cool"); // get secret from a source with the provided identifier
	console.log(my_secret);
	await secrets.get("else"); // get secret from a source with the provided identifier
	console.log(my_secret);
}
