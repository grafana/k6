import { group, check } from "speedboat";
import http from "speedboat/http";

export let options = {
	thresholds: {
		http_req_duration: ["avg<=100"],
	}
};

export default function() {
	group("front page", function() {
		check(http.get("http://localhost:8080/"), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("stylesheet", function() {
		check(http.get("http://localhost:8080/style.css"), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("image", function() {
		check(http.get("http://localhost:8080/teddy.jpg"), {
			"status is 200": (res) => res.status === 200,
		});
	});
}
