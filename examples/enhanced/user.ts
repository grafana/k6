interface User {
	name: string;
	id: number;
}

class UserAccount implements User {
	name: string;
	id: number;

	constructor(name: string) {
		this.name = name;
		this.id = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
	}
}

function newUser(name: string): User {
	return new UserAccount(name);
}

export { User, newUser };
