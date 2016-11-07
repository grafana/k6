import { check } from "speedboat";
import http from "speedboat/http";

export default function() {
	check(http.get("http://localhost:8080/"), {
		"status is 200": (v) => v.status === 200,
	});
}
