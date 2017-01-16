import { group, check } from "k6";
import http from "k6/http";

export let options = {
	thresholds: {
		http_req_duration: ["avg<=100*ms"],
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
