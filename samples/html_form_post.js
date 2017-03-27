import http from "k6/http";
import { check } from "k6";

// Our form data, to be URL-encoded and POSTed
const form_data = {
    name: "Test Name",
    telephone: "123456789",
    email: "test@example.com",
    comment: "Hello world!",
    topping: [
        'onion',
        'bacon',
        'cheese'
    ]
};

export default function() {
    // Passing an object as the data parameter will automatically form-urlencode it
    let r = http.post("http://httpbin.org/post", form_data);

    // Verify response
    let j = r.json()["form"];
    check(r, {
        "status is 200": (r) => r.status === 200,
        "has correct name": (r) => j["name"] === form_data.name,
        "has correct telephone number": (r) => j["telephone"] === form_data.telephone,
        "has correct email": (r) => j["email"] === form_data.email,
        "has correct comment": (r) => j["comment"] === form_data.comment,
        "has correct toppings": (r) => j["topping"] === form_data.topping
    });
}
