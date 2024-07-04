import { User, newUser } from "./user.ts";

export default () => {
	const user: User = newUser("John");
	console.log(user);
};
