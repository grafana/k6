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
    let res = http.post("http://httpbin.org/post", form_data);

    // Verify response
    check(res, {
        "status is 200": (r) => r.status === 200,
        "has correct name": (r) => r.json().form.name === form_data.name,
        "has correct telephone number": (r) => r.json().form.telephone === form_data.telephone,
        "has correct email": (r) => r.json().form.email === form_data.email,
        "has correct comment": (r) => r.json().form.comment === form_data.comment,
        "has correct toppings": (r) => JSON.stringify(r.json().form.topping) === JSON.stringify(form_data.topping)
    });
}
