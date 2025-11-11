// k6 run --secret-source=mock=default,cool="cool secret" --secret-source=mock=name=another,cool="not cool secret" multi-source.test.js
import secrets from "k6/secrets";

export default async () => {
	const my_secret = await secrets.get("cool"); // get secret from a source with the provided identifier
	const anothersource = secrets.source("another")
	const my_other_secret = await anothersource.get("cool");
	console.log("Secret from default source ", my_secret)
	console.log("Secret from another source ", my_other_secret)
	console.log("Are they equal? ", my_other_secret== my_secret)
}
