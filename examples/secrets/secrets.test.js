// k6 run --secret-source=file=file.secret secrets.test.js
import secrets from "k6/secrets";

export default () => {
	const my_secret = secrets.get("cool"); // get secret from a source with the provided identifier
	console.log(my_secret);
	secrets.get("else"); // get secret from a source with the provided identifier
	console.log(my_secret);
}
