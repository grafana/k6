// This script tests a simple webshop hosted at Pantheon (https://pantheon.io/).
// Pantheon are our friends and don't mind us doing this, but please remember not to test
// any sites without the owner's consent. They can get quite grumpy and/or ban you.

// TODO: Parse the DOM properly; don't parse HTML with regex. That way lies madness.
// This will be implemented in the VU API very, very soon.
function extractForm(html, idPrefix) {
	var findFormRE = new RegExp('<form [^<]*id="(' + idPrefix + '[^"]*)[^>]*>(?:.|\n)*<\/form>', 'gm');
	var findInputRE = /<input[^<]*name="([^"]+)"[^<]*value="([^"]+)"[^>]*\/>/gm;

	var formMatch = null;
	while ((formMatch = findFormRE.exec(html)) !== null) {
		// Copy fields to the request body.
		var body = {};
		var fieldMatch = null;
		while ((fieldMatch = findInputRE.exec(formMatch[0])) !== null) {
			body[fieldMatch[1] = fieldMatch[2]];
		}

		return body;
	}

	return null;
}

// Add an item to the user's cart.
function addToCart(url, res) {
	$log.info("adding item to cart", { url: url });

	// Order a random quantity of the item.
	body = extractForm(res.body, "commerce-cart-add-to-cart-form-");
	if (body === null) {
		throw new Error("couldn't find add-to-cart form on: " + url);
	}
	body["quantity"] = (Math.floor(10 * Math.random()) + 1).toString();

	// Put it in the cart.
	var postRes = $http.post(url, body);
	$assert.equal(200, postRes.status, "wrong status from adding to cart");
}

// Reads a list of URLs and randomly clicks links.
function readList(urls) {
	var added = 0;

	// Examine every result, and randomly click some links. Note that we're not
	// looking at the actual response from the server; parsing HTML is a lot of work.
	for (var i = 0; i < urls.length; i++) {
		// Take a moment to have a good look at each result.
		$vu.sleep(2 * Math.random() + 1);

		// If the item looks interesting, click it.
		if (Math.random() < 0.5) {
			var url = urls[i];
			var res = $http.get(url);
			$assert.equal(200, res.status, "wrong status from url: " + url);
			$vu.sleep(15 * Math.random() + 5);

			// 30% chance of this item going in the cart.
			if (Math.random() < 0.3) {
				addToCart(url, res);
				$vu.sleep(2 * Math.random() + 1);
				added += 1;
			}
		}
	}

	$log.debug("added", { added: 1 });
	return added;
}

// Browse the store through one or more of a couple of possible searches, and open items
// that look interesting in tabs to look at later.
function browseBySearch() {
	// Possible searches and their results.
	var searches = {
		"cap": [
			"https://dev-li-david.pantheonsite.io/hats/commerce-guys-baseball-cap",
			"https://dev-li-david.pantheonsite.io/hats/drupal-commerce-ski-cap",
		],
		"hoodie": [
			"https://dev-li-david.pantheonsite.io/tops/guy-hoodie",
			"https://dev-li-david.pantheonsite.io/tops/drupal-commerce-hoodie",
		],
		"bag": [
			"https://dev-li-david.pantheonsite.io/bags-cases/go-green-drupal-commerce-reusable-tote-bag",
			"https://dev-li-david.pantheonsite.io/bags-cases/drupal-commerce-messenger-bag",
			"https://dev-li-david.pantheonsite.io/bags-cases/commerce-guys-laptop-bag",
		],
	};

	var added = 0;

	// Do a random number of searches, up to a maximum of as many as we have prepared.
	for (var blah = 0; blah < Math.floor(Math.random() * (Object.keys(searches).length - 1)) + 1; blah++) {
		// Pick a random search, then delete it to prevent it from being made twice.
		var keys = Object.keys(searches);
		var key = keys[Math.floor(Math.random() * keys.length)];
		var results = searches[key];
		delete searches[key];

		// Send the search to the server.
		$log.info("performing search", { search: key });
		var searchRes = $http.get("https://dev-li-david.pantheonsite.io/products", { search_api_views_fulltext: key });
		$assert.equal(200, searchRes.status, "wrong status from search (keyword: " + key + ")");

		// Go through the results.
		added += readList(results);
	}

	return added;
}

function browseByCategories() {
	// Categories and their contents.
	var categories = {
		"https://dev-li-david.pantheonsite.io/collection/carry": [
			"https://dev-li-david.pantheonsite.io/bags-cases/commerce-guys-laptop-bag",
			"https://dev-li-david.pantheonsite.io/bags-cases/drupal-commerce-messenger-bag",
			"https://dev-li-david.pantheonsite.io/bags-cases/go-green-drupal-commerce-reusable-tote-bag",
		],
		"https://dev-li-david.pantheonsite.io/collection/drink": [
			"https://dev-li-david.pantheonsite.io/drinks/drupal-commerce-wake-you",
			"https://dev-li-david.pantheonsite.io/drinks/guy-mug",
			"https://dev-li-david.pantheonsite.io/drinks/guy-h20",
		],
		"https://dev-li-david.pantheonsite.io/collection/geek-out": [
			"https://dev-li-david.pantheonsite.io/bags-cases/drupal-commerce-iphone-case",
			"https://dev-li-david.pantheonsite.io/storage-devices/commerce-guys-usb-key",
		],
		"https://dev-li-david.pantheonsite.io/collection/wear": [
			"https://dev-li-david.pantheonsite.io/hats/commerce-guys-baseball-cap",
			"https://dev-li-david.pantheonsite.io/hats/drupal-commerce-ski-cap",
			"https://dev-li-david.pantheonsite.io/shoes/drupal-commerce-ready-beach",
			"https://dev-li-david.pantheonsite.io/shoes/drupal-commerce-ready-court",
			"https://dev-li-david.pantheonsite.io/tops/guy-hoodie",
			"https://dev-li-david.pantheonsite.io/tops/drupal-commerce-hoodie",
			"https://dev-li-david.pantheonsite.io/tops/drupal-commerce-knit-long-sleeve",
			"https://dev-li-david.pantheonsite.io/tops/guy-short-sleeve-tee",
			"https://dev-li-david.pantheonsite.io/tops/commerce-guys-long-sleeve-henley",
			"https://dev-li-david.pantheonsite.io/tops/commerce-guys-polo",
			"https://dev-li-david.pantheonsite.io/tops/commerce-guys-womens-tee",
			"https://dev-li-david.pantheonsite.io/tops/drupal-commerce-womens-tee",
		]
	};

	var added = 0;

	// Click a number of random categories.
	for (var blah = 0; blah < Math.floor(Math.random() * (Object.keys(categories).length - 1)) + 1; blah++) {
		// Pick a category, then remove it from the list to prevent reuse.
		var keys = Object.keys(categories);
		var cat = keys[Math.floor(Math.random() * keys.length)];
		var results = categories[cat];
		delete categories[cat];

		// Send a request to the category.
		$log.info("browsing category", { cat: cat });
		catRes = $http.get(cat);
		$assert.equal(200, catRes.status, "wrong status from category: " + cat);

		// Read the results.
		added += readList(results);
	}

	return added;
}

// Use a top-level function wrapper to allow us to return from it. If you don't need to
// return from your script at any point, you can skip the wrapper - VU scopes are isolated.
(function() {
	// The VU first goes to the front page, and takes about 10s to look at it.
	// TODO: Load static resources here. We just need to fix setMaxConnsPerHost() before
	// we can do that over non-HTTP/2 connections without killing the client machine.
	var res = $http.get("https://dev-li-david.pantheonsite.io/");
	$assert.equal(200, res.status, "wrong status from front page");
	$vu.sleep(20 * Math.random() + 10);

	// Some may not even be interested in what you're selling, and leave immediately;
	// Google Analytics calls this "bounce rate". We'll set it at around 15%.
	if (Math.random() < 0.15) {
		$log.debug("bouncing");
		return
	}

	// They may decide to read the featured blog post.
	if (Math.random() < 0.2) {
		$log.info("reading featured blog post")
		var res = $http.get("https://dev-li-david.pantheonsite.io/blog/social-logins-made-simple");
		$assert.equal(200, res.status, "wrong status from the featured blog post");
		$vu.sleep(10 * Math.random() + 5);
	}

	// ~20% will read up on the store a before browsing further. Knowledge is power.
	if (Math.random() < 0.2) {
		$log.info("reading about page")
		var res = $http.get("https://dev-li-david.pantheonsite.io/about");
		$assert.equal(200, res.status, "wrong status from about page");
		$vu.sleep(10 * Math.random() + 5);
	}

	// About 10% will just be done here, 30% will search for something, the remaining
	// 60% will go browse random items through categories categories.
	var num = Math.random();
	if (num < 0.1) {
		$log.info("done");
		return;
	} else if (num < 0.4) {
		$log.info("browsing by search");
		if (browseBySearch() === 0) {
			$log.info("saw nothing interesting");
			return;
		}
	} else {
		$log.info("browsing by categories");
		if (browseByCategories() === 0) {
			$log.info("saw nothing interesting");
			return;
		}
	}

	// Proceed to checkout.
	$log.info("proceeding to checkout");
	var cartRes = $http.get("https://dev-li-david.pantheonsite.io/cart");
	$assert.equal(200, cartRes.status, "wrong status from cart page");
	$vu.sleep(8 * Math.random() + 2);

	// Submit!
	var cartFields = extractForm(cartRes.body, "views-form-commerce-cart-form-default");
	if (cartFields === null) {
		$log.info("nothing in the cart!");
		return;
	}
	cartFields["op"] = "Checkout";
	var loginRedirectRes = $http.post("https://dev-li-david.pantheonsite.io/cart", cartFields, { follow: true });
	$assert.equal(200, loginRedirectRes.status, "wrong status from login redirect");

	// Log in...
	var loginFields = extractForm(loginRedirectRes.body, "user-login");
	cartFields["name"] = "testaccount";
	cartFields["pass"] = "password";
	var loginRes = $http.post("https://dev-li-david.pantheonsite.io/user/login", loginFields, { follow: true });
	$assert.equal(200, cartRes.status, "wrong status from login");

	// We could continue here in the same way, but it should be enough to go this far.
})()
