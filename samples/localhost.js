import { group, check } from "k6";
import http from "k6/http";

export let options = {
	thresholds: {
		'http_req_duration{kind:html}': ["avg<=10"],
		'http_req_duration{kind:css}': ["avg<=10"],
		'http_req_duration{kind:img}': ["avg<=100"],
	}
};

export default function() {
	group("front page", function() {
		check(http.get("http://localhost:8080/", null, {
			tags: {'kind': 'html' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("stylesheet", function() {
		check(http.get("http://localhost:8080/style.css", null, {
			tags: {'kind': 'css' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("image", function() {
		check(http.get("http://localhost:8080/teddy.jpg", null, {
			tags: {'kind': 'img' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
}
