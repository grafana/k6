import { http } from "speedboat";

export let options = {
	vus: 100,
};

export default function() {
	http.get("http://localhost:8080");
};
