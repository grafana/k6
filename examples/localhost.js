import { group, check } from "k6";
import http from "k6/http";

export let options = {
	thresholds: {
		'http_req_duration{kind:html}': ["avg<=10"],
		'http_req_duration{kind:css}': ["avg<=10"],
		'http_req_duration{kind:img}': ["avg<=100"],
		'http_reqs': ["rate>100"],
	}
};

export default function() {
	group("front page", function() {
		check(http.get("http://localhost:8080/", {
			tags: {'kind': 'html' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("stylesheet", function() {
		check(http.get("http://localhost:8080/style.css", {
			tags: {'kind': 'css' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
	group("image", function() {
		check(http.get("http://localhost:8080/teddy.jpg", {
			tags: {'kind': 'img' },
		}), {
			"status is 200": (res) => res.status === 200,
		});
	});
}
