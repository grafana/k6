import { group, check, sleep } from "k6";
import http from "k6/http";

export let options = {
    maxRedirects: 10
};

//
// This is an advanced k6 script sample that simulates users
// logging into an e-commerce site and purchasing things there.
//

// Emulate Chrome on MacOS
let defaultheaders = {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.95 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
//    "Accept-Encoding": "gzip, deflate, sdch",  --- we do not want compressed content
    "Accept-Language": "en-US,en;q=0.8,sv;q=0.6"
};

// Create a version of our default headers with a static If-Modified-Since header. We use this
// to ask for cached resources where we want the server to return a 304 rather than the actual content.
let cacheheaders = defaultheaders;
cacheheaders["If-Modified-Since"] = "Tue, 21 Feb 2017 14:24:11 GMT";

// Read our username and password from the users.json file, which should have the following format:
// [ { "username": "joe", "password": "secret" }, { "username": "anne", "password": "alsosecret" }, ... ]
// Use our unique VU id number to index the contents of the file and find our particular user data
//
//let users = JSON.parse(open("users.json"));
let users = [ 
    { "username": "testuser1", "password": "testuser1" }
];

// Base URL for the site
let baseurl = "http://dev-li-david.pantheonsite.io";

// A few globals used in the purchase flow
let form_build_id = "";
let form_id = "";
let form_token= "";
let checkout_url = "";
let referer = "";

// Simpler, alternate main loop that logs in, performs a purchase and logs out again.
// export default function() {
//    loginpage();
//    do_login(username, password);
//    drupalbag();
//    add_drupalbag();
//    cartreview();
//    cartsubmit();
//    checkout();
//    shipping();
//    review_submit();
//    logout();
//}

// main loop
export default function() {
    // 1. Load home page
    firstpage();
    // Load dependencies (images etc). Simulate not having anything cached.
    page_dependencies(false);
    // 0-30 second user think time
    thinktime(30);

    // 2. Load login page and save the hidden form field needed to logon
    loginpage();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time. Takes a while to type in username and password.
    thinktime(30);

    // 3. Perform login
    do_login(users[__VU-1]["username"], users[__VU-1]["password"]);
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 4. Look at "carry" product section
    carrypage();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 5. Choose "drupalbag" product
    drupalbag();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 6. Add product to our shopping cart
    add_drupalbag();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 7. View our shopping cart
    cartreview();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 8. Proceed to checkout
    cartsubmit();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);
    
    // 9. Perform checkout
    checkout();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);
    
    // 10. Select shipping method
    shipping();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 11. Review and submit order
    review_submit();
    // Ask server for updated (If-Modified-Since) dependencies
    page_dependencies(true);
    // User think time.
    thinktime(30);

    // 12. Logout
    logout();
    // User thinktime
    thinktime(30);
}

function thinktime(t) {
    return;
    sleep(t * Math.random());
}

// This function loads the home page HTML
function firstpage() {
    let params = { "headers": defaultheaders };
    let url = baseurl + "/";
    // Load main HTML
    let response = http.get(url, null, params);
    check(response, {
        "1: first page content OK": (res) => res.html("title").text() === 'Welcome to David li commerce-test | David li commerce-test'
    }) || console.log("First page content invalid");
    // We always update the "Referer" header to contain the most recently accessed URL
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// This function loads the login page, where the login form is (wher you enter your
// username and password to login)
function loginpage() {
    let params = { "headers": defaultheaders };
    let url = baseurl + "/user/login";
    let response = http.get(url, null, params);
    check(response, {
        "2: login page OK": (res) => res.html("title").text() === 'User account | David li commerce-test'
    }) || console.log("Login page content invalid");
    // Now we look for some hidden form fields, and extract their values so we can use them 
    // when submitting the form later on:
    // <input type="hidden" name="form_build_id" value="form-euqedAF5cQGec_Z9qqgjNMQsMzNAkiF37BGokRobLNg" />
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// This function performs a login POST operation to authenticate the user.
// It uses the previously stored hidden form fields when submitting the form.
function do_login(username, password) {
    let headers = defaultheaders;
    // We set the content type specifically for form POSTs
    headers["Content-Type"] = "application/x-www-form-urlencoded";
    let params = { "headers": headers };
    let url = baseurl + "/user/login";
    formdata = {
        "name": username,
        "pass": password,
        "form_build_id": form_build_id,
        "form_id": "user_login",
        "op": "Log in"
    };
    let response = http.post(url, formdata, params);
    // verify login succeeded
    check(response, {
        "3: login succeeded": (res) => res.url === ( baseurl + "/users/" + username)
    }) || console.log("Login failed!  Effective URL was " + response.url);
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// This function loads the /collection/carry page
function carrypage() {
    let params = { "headers": defaultheaders }
    let url = baseurl + "/collection/carry";
    let response = http.get(url, null, params);
    check(response, {
        "4: carry page OK": (res) => res.html("title").text() === 'To carry | David li commerce-test'
    }) || console.log("Carry page content invalid");
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// And here we check out the "drupal bag", going to its product page
function drupalbag() {
    let params = { "headers": defaultheaders };
    let url = baseurl + "/bags-cases/drupal-commerce-messenger-bag";
    let response = http.get(url, null, params);
    check(response, {
        "5: drupal bag page OK": (res) => res.html("title").text() === 'Drupal Commerce Messenger Bag | David li commerce-test'
    }) || console.log("Drupal bag page content invalid");
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    form_token = response.body.match('name="form_token" value="(.*)"')[1];
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Then we add the Drupal bag to our shopping cart
function add_drupalbag() {
    let headers = defaultheaders;
    headers["Content-Type"] = "application/x-www-form-urlencoded";
    //headers["Origin"] = baseurl;
    params = { "headers": headers };
    let url = baseurl + "/bags-cases/drupal-commerce-messenger-bag";
    let formdata = {
        "product_id": 2,
        "form_build_id": form_build_id,
        "form_id": form_id,
        "form_token": form_token,
        "quantity": 1,
        "op": "Add to cart"
    };    
    let response = http.post(url, formdata, params);
    // verify add to cart succeeded
    check(response, {
        "6: add to cart succeeded": (res) => res.body.includes('Item successfully added to your cart')
    }) || console.log("Add to cart failed");
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Then we click the checkout link to go to our shopping cart
function cartreview() {
    let params = { "headers": defaultheaders };
    let url = baseurl + "/cart";
    let response = http.get(url, null, params);
    check(response, {
        "7: shopping cart page OK": (res) => res.html("title").text() === 'Shopping cart | David li commerce-test'
    }) || console.log("Shopping cart page content invalid");
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_token = response.body.match('name="form_token" value="(.*)"')[1];
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    checkout_url = response.url;
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Go to checkout
function cartsubmit() {
    let headers = defaultheaders;
    headers["Content-Type"] = "application/x-www-form-urlencoded";
    //headers["Origin"] = baseurl;
    let params = { "headers": headers };
    let url = baseurl + "/cart";
    let formdata = {
        "form_build_id": form_build_id,
        "form_token": form_token,
        "form_id": form_id,
        "edit_quantity[0]": 1,
        "op": "Checkout"
    };
    let response = http.post(url, formdata, params);
    check(response, {
        "8: cart submit succeeded": (res) => res.url.includes("/checkout/")
    }) || console.log("Cart submit failed");
    // This POST redirects to checkout page, which has a dynamic path, e.g "/checkout/7"
    // so we save the redirected URL in a global variable.
    checkout_url = response.url;
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_token = response.body.match('name="form_token" value="(.*)"')[1];
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Enter billing address etc
function checkout() {
    let headers = defaultheaders
    headers["Content-Type"] = "application/x-www-form-urlencoded"
    params = { "headers": headers }
    // We use the URL we saved earlier
    let url = checkout_url;
    formdata = {
        "customer_profile_billing[commerce_customer_address][und][0][country]": "SE",
        "customer_profile_billing[commerce_customer_address][und][0][name_line]": "Mr Test",
        "customer_profile_billing[commerce_customer_address][und][0][thoroughfare]": "Gotgatan 14",
        "customer_profile_billing[commerce_customer_address][und][0][premise]": "",
        "customer_profile_billing[commerce_customer_address][und][0][postal_code]": "11846",
        "customer_profile_billing[commerce_customer_address][und][0][locality]": "Stockholm",
        "customer_profile_shipping[commerce_customer_profile_copy]": "1",
        "form_build_id": form_build_id,
        "form_token": form_token,
        "form_id": form_id,
        "op": "Continue to next step"
    }
    let response = http.post(url, formdata, params);
    // verify checkout step 1 succeeded
    check(response, {
        "9: checkout succeeded": (res) => res.url === (checkout_url + "/shipping")
    }) || console.log("Checkout failed!");
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_token = response.body.match('name="form_token" value="(.*)"')[1];     
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Checkout step 2: choose shipping option
function shipping() {
    let headers = defaultheaders;
    headers["Content-Type"] = "application/x-www-form-urlencoded";
    params = { "headers": headers };
    let url = checkout_url + "/shipping";
    formdata = {
        "commerce_shipping[shipping_service]": "express_shipping",
        "form_build_id": form_build_id,
        "form_token": form_token,
        "form_id": form_id,
        "op": "Continue to next step"
    }
    let response = http.post(url, formdata, params);
    // verify checkout step 2 succeeded
    check(response, {
        "10: select shipping succeeded": (res) => res.url === (checkout_url + "/review")
    }) || console.log("Select shipping failed!");
    form_build_id = response.body.match('name="form_build_id" value="(.*)"')[1];
    form_token = response.body.match('name="form_token" value="(.*)"')[1];    
    form_id = response.body.match('name="form_id" value="(.*)"')[1];
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Checkout step 3: review and submit order
function review_submit() {
    let headers = defaultheaders;
    headers["Content-Type"] = "application/x-www-form-urlencoded";
    params = { "headers": headers };
    let url = checkout_url + "/review";
    formdata = {
        "commerce_payment[payment_method]": "commerce_payment_example|commerce_payment_commerce_payment_example",
        "commerce_payment[payment_details][credit_card][number]": "4111111111111111",
        "commerce_payment[payment_details][credit_card][exp_month]": "03",
        "commerce_payment[payment_details][credit_card][exp_year]": "2019",
        "form_build_id": form_build_id,
        "form_token": form_token,
        "form_id": form_id,
        "op": "Continue to next step"
    }
    let response = http.post(url, formdata, params);
    // if this POST succeeds, it will redirect to e.g. /checkout/7/payment
    // /checkout/7/payment, in turn, will redirect to /checkout/7/paypal_ec
    // /checkout/7/paypal_ec, in turn, will redirect to /checkout/7/complete
    check(response, {
        "11: checkout complete": (res) => res.html("h1").text() === "Checkout complete"
    }) || console.log("Checkout review-submit failed");
    defaultheaders["Referer"] = cacheheaders["Referer"] = response.url;
}

// Finally, we log out our user
function logout() {
    let headers = defaultheaders;
    let params = { "headers": headers };
    let url = baseurl + "/user/logout";
    let response = http.get(url, null, params);
    check(response, {
        "12: logout succeeded": (res) => res.body.includes('<a href="/user/login">Log in')
    }) || console.log("Logout failed");
}


// page_dependencies() loads a bunch of dependencies (images, css files etc.)
// either using "defaultheaders" or "cacheheaders", where the latter contains
// an If-Modified-Since header that allows the server to just respond with 304
// (content has not been modified) instead of sending the actual content.
//
// We use this function as "filler" in between the requests that are part of the
// user flow, in order to behave like a real browser would. We always ask for the
// same set of files, while in reality the files asked for varies slightly 
// between one page and another. The overlap is substantial, however, and it is
// likely that this simplification is not going to affect results in the
// slightest.
//
function page_dependencies(cached) {
    let params = { "headers": defaultheaders };
    if (cached) {
        params = { "headers": cacheheaders };
    }
    let responses = http.batch([
        { "url": baseurl + "/modules/system/system.base.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/system/system.menus.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/system/system.messages.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/system/system.theme.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/cloud_zoom/css/cloud_zoom.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/jquery.ui.core.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/jquery.ui.theme.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/jquery_ui_spinner/ui.spinner.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/comment/comment.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce_add_to_cart_confirmation/css/commerce_add_to_cart_confirmation.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_menus/commerce_kickstart_menus.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/date/date_api/date.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/date/date_popup/themes/datepicker.1.7.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/fences/field.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/node/node.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/modules/user/user.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/views/css/views.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/ctools/css/ctools.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce/modules/line_item/theme/commerce_line_item.theme.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce/modules/product/theme/commerce_product.theme.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce_fancy_attributes/commerce_fancy_attributes.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-reset.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-mobile.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-alpha.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/formalize.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-text.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-branding.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-menu.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-forms.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/global.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce_kickstart_style.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default-narrow.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default-narrow.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/grid/alpha_default/narrow/alpha-default-narrow-24.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default-normal.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default-normal.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/grid/alpha_default/normal/alpha-default-normal-24.css?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/misc/jquery.js?v=1.4.4", "method": "GET", "params": params },
        { "url": baseurl + "/misc/jquery.once.js?v=1.2", "method": "GET", "params": params },
        { "url": baseurl + "/misc/drupal.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/jquery.ui.core.min.js?v=1.8.7", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/jquery.ui.widget.min.js?v=1.8.7", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/cloud-zoom/cloud-zoom.1.0.3.min.js?v=1.0.3", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/cloud_zoom/js/cloud_zoom.js?v=1.0.3", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/jquery_expander/jquery.expander.min.js?v=1.4.2", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/jquery_ui_spinner/ui.spinner.min.js?v=1.8", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/selectnav.js/selectnav.min.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce_add_to_cart_confirmation/js/commerce_add_to_cart_confirmation.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_search/commerce_kickstart_search.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/service_links/js/twitter_button.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/service_links/js/facebook_like.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/service_links/js/google_plus_one.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/contrib/commerce_fancy_attributes/commerce_fancy_attributes.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_product_ui/commerce_kickstart_product_ui.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/js/omega_kickstart.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/js/jquery.formalize.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega/omega/js/omega-mediaqueries.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/js/commerce_kickstart_theme_custom.js?olqap9", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/logo.png", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/styles/product_full/public/messenger-1v1.jpg?itok=hPe-GkYY", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/styles/product_thumbnail/public/messenger-1v1.jpg?itok=cXkqMlMc", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/styles/product_thumbnail/public/messenger-1v2.jpg?itok=yyhLIuCD", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/styles/product_thumbnail/public/messenger-1v3.jpg?itok=uQsNvRiQ", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/styles/product_thumbnail/public/messenger-1v4.jpg?itok=ns9kHz1T", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/bg.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/picto_cart.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_magnifying_glass.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_product_attributes_bottom.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_product_attributes_top.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_add_to_cart.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/bg_block_footer_title.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_facebook.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_twitter.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_pinterest.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_mastercard.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_paypal.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_visa_premier.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_american_express.png", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/images/ui-bg_glass_75_e6e6e6_1x400.png", "method": "GET", "params": params },
        { "url": baseurl + "/misc/ui/images/ui-icons_888888_256x240.png", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/btn_read_more.png", "method": "GET", "params": params },
        { "url": baseurl + "/sites/default/files/messenger-1v1.jpg", "method": "GET", "params": params },
        { "url": baseurl + "/profiles/commerce_kickstart/libraries/cloud-zoom/blank.png", "method": "GET", "params": params }
    ]);
}






