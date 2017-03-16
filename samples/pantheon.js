import { group, check, sleep, fail } from "k6";
import http from "k6/http";

export let options = {
	maxRedirects: 10,
}

const baseURL = "https://dev-li-david.pantheonsite.io";

const credentials = [
	{ username: "testuser1", password: "testuser1" },
];

const staticAssets = [
	baseURL + "/modules/system/system.base.css?olqap9",
	baseURL + "/modules/system/system.menus.css?olqap9",
	baseURL + "/modules/system/system.messages.css?olqap9",
	baseURL + "/modules/system/system.theme.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/cloud_zoom/css/cloud_zoom.css?olqap9",
	baseURL + "/misc/ui/jquery.ui.core.css?olqap9",
	baseURL + "/misc/ui/jquery.ui.theme.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/libraries/jquery_ui_spinner/ui.spinner.css?olqap9",
	baseURL + "/modules/comment/comment.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce_add_to_cart_confirmation/css/commerce_add_to_cart_confirmation.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_menus/commerce_kickstart_menus.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/date/date_api/date.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/date/date_popup/themes/datepicker.1.7.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/fences/field.css?olqap9",
	baseURL + "/modules/node/node.css?olqap9",
	baseURL + "/modules/user/user.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/views/css/views.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/ctools/css/ctools.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce/modules/line_item/theme/commerce_line_item.theme.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce/modules/product/theme/commerce_product.theme.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce_fancy_attributes/commerce_fancy_attributes.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-reset.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-mobile.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/alpha-alpha.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/formalize.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-text.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-branding.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-menu.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/css/omega-forms.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/global.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce_kickstart_style.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default-narrow.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default-narrow.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/grid/alpha_default/narrow/alpha-default-narrow-24.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/css/omega-kickstart-alpha-default-normal.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/css/commerce-kickstart-theme-alpha-default-normal.css?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/alpha/css/grid/alpha_default/normal/alpha-default-normal-24.css?olqap9",
	baseURL + "/misc/jquery.js?v=1.4.4",
	baseURL + "/misc/jquery.once.js?v=1.2",
	baseURL + "/misc/drupal.js?olqap9",
	baseURL + "/misc/ui/jquery.ui.core.min.js?v=1.8.7",
	baseURL + "/misc/ui/jquery.ui.widget.min.js?v=1.8.7",
	baseURL + "/profiles/commerce_kickstart/libraries/cloud-zoom/cloud-zoom.1.0.3.min.js?v=1.0.3",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/cloud_zoom/js/cloud_zoom.js?v=1.0.3",
	baseURL + "/profiles/commerce_kickstart/libraries/jquery_expander/jquery.expander.min.js?v=1.4.2",
	baseURL + "/profiles/commerce_kickstart/libraries/jquery_ui_spinner/ui.spinner.min.js?v=1.8",
	baseURL + "/profiles/commerce_kickstart/libraries/selectnav.js/selectnav.min.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce_add_to_cart_confirmation/js/commerce_add_to_cart_confirmation.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_search/commerce_kickstart_search.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/service_links/js/twitter_button.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/service_links/js/facebook_like.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/service_links/js/google_plus_one.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/contrib/commerce_fancy_attributes/commerce_fancy_attributes.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/modules/commerce_kickstart/commerce_kickstart_product_ui/commerce_kickstart_product_ui.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/js/omega_kickstart.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/js/jquery.formalize.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega/omega/js/omega-mediaqueries.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/js/commerce_kickstart_theme_custom.js?olqap9",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/logo.png",
	baseURL + "/sites/default/files/styles/product_full/public/messenger-1v1.jpg?itok=hPe-GkYY",
	baseURL + "/sites/default/files/styles/product_thumbnail/public/messenger-1v1.jpg?itok=cXkqMlMc",
	baseURL + "/sites/default/files/styles/product_thumbnail/public/messenger-1v2.jpg?itok=yyhLIuCD",
	baseURL + "/sites/default/files/styles/product_thumbnail/public/messenger-1v3.jpg?itok=uQsNvRiQ",
	baseURL + "/sites/default/files/styles/product_thumbnail/public/messenger-1v4.jpg?itok=ns9kHz1T",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/bg.png",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/picto_cart.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_magnifying_glass.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_product_attributes_bottom.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_product_attributes_top.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/bg_add_to_cart.png",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/bg_block_footer_title.png",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_facebook.png",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_twitter.png",
	baseURL + "/profiles/commerce_kickstart/themes/commerce_kickstart_theme/images/icon_pinterest.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_mastercard.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_paypal.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_visa_premier.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/picto_american_express.png",
	baseURL + "/misc/ui/images/ui-bg_glass_75_e6e6e6_1x400.png",
	baseURL + "/misc/ui/images/ui-icons_888888_256x240.png",
	baseURL + "/profiles/commerce_kickstart/themes/contrib/omega_kickstart/images/btn_read_more.png",
	baseURL + "/sites/default/files/messenger-1v1.jpg",
	baseURL + "/profiles/commerce_kickstart/libraries/cloud-zoom/blank.png",
];

function doFrontPage() {
	// Load the front page.
	check(http.get(baseURL + "/"), {
		"title is correct": (res) => res.html("title").text() == "Welcome to David li commerce-test | David li commerce-test",
	});

	// Load static assets.
	group("static", function() { http.batch(staticAssets); });
}

function doLogin() {
	// Request the login page.
	let res = http.get(baseURL + "/user/login");
	check(res, {
		"title is correct": (res) => res.html("title").text() == "User account | David li commerce-test",
	});

	// TODO: Add attr() to k6/html!
	// Extract hidden input fields.
	let form_build_id = res.body.match('name="form_build_id" value="(.*)"')[1];
	let form_id = res.body.match('name="form_id" value="(.*)"')[1];

	group("login", function() {
		// Pick a random set of credentials.
		let creds = credentials[Math.floor(Math.random()*credentials.length)];

		// Send the login request.
		let res = http.post(
			baseURL + "/user/login", {
				name: creds.username,
				pass: creds.password,
				form_build_id: form_build_id,
				form_id: "user_login",
				op: "Log in",
			}, {
				headers: { "Content-Type": "application/x-www-form-urlencoded" },
			},
		);
		check(res, {
			"login succeeded": (res) => res.url == `${baseURL}/users/${creds.username}`,
		}) || fail("login failed");
	});
}

function doCategory(url, title, products) {
	check(http.get(url), {
		"title is correct": (res) => res.html("title").text() == title,
	});

	let prodNames = Object.keys(products);
	let prodName = prodNames[Math.floor(Math.random()*prodNames.length)];
	group(prodName, function() { doProductPage(...products[prodName]) });
}

function doProductPage(url, title, want) {
	check(http.get(url), {
		"title is correct": (res) => res.html("title").text() == title,
	});
}

export default function() {
	group("front page", doFrontPage);
	// sleep(30);

	group("login page", doLogin);
	// sleep(30);

	let categories = {
		"To Carry": [
			`${baseURL}/collection/to-carry`,
			"To carry | David li commerce-test",
			{
				"Drupal Bag": [
					`${baseURL}/bags-cases/drupal-commerce-messenger-bag`,
					"Drupal Commerce Messenger Bag | David li commerce-test",
					true,
				],
			}
		],
	};
	for (name in categories) {
		group(name, function() { doCategory(...categories[name]); });
	}
}
