import { group, check, sleep } from 'k6';
import http from 'k6/http';

// Version: 1.2
// Creator: BrowserMob Proxy

export let options = {
    maxRedirects: 0,
};

export default function() {

	group("Page 0 - Page 0 ţ€$ţɨɲǥ µɲɨȼ๏ď€ ɨɲ Ќ6 \" \x00\n\t♥\u2028", function() {
		let res, redirectUrl, json;
		// Request #0
		res = http.post("https://some-host.example.com/checkout/v3/orders",
			`{
				"locale": "sv-SE",
				"merchant_urls": {
					"checkout": "https://some-fourth-host.example.com/v1/redirect/checkout",
					"confirmation": "https://some-fourth-host.example.com/v1/redirect/confirm",
					"push": "https://some-fourth-host.example.com/v1/callback/push/{checkout.order.id}?merchant_id=smi-merchant-all-validation\u0026env=perf",
					"terms": "https://some-fourth-host.example.com/v1/redirect/terms"
				},
				"options": {},
				"order_amount": 16278,
				"order_lines": [
					{
						"image_url": "https://s3-eu-west-1.amazonaws.com/s3.example.net/my/system-test/images/7.jpg",
						"name": "Mediokra Betong Lampa. Tangentbord",
						"product_url": "http://aufderharluettgen.info/haven",
						"quantity": 1,
						"quantity_unit": "kg",
						"reference": "jkwedq9f6t",
						"tax_rate": 800,
						"total_amount": 16278,
						"total_discount_amount": 0,
						"total_tax_amount": 1206,
						"type": "physical",
						"unit_price": 16278
					}
				],
				"order_tax_amount": 1206,
				"purchase_country": "se",
				"purchase_currency": "SEK",
				"shipping_countries": ["AD", "AE", "AG", "AI", "AL", "AM", "AQ", "AR", "AS", "AT", "AU", "AW", "AX", "AZ", "BB", "BD", "BE", "BF", "BG", "BH", "BJ", "BL", "BM", "BN", "BO", "BQ", "BR", "BS", "BT", "BV", "BW", "BZ", "CA", "CC", "CH", "CK", "CL", "CM", "CO", "CR", "CU", "CV", "CW", "CX", "CY", "CZ", "DE", "DJ", "DK", "DM", "DO", "DZ", "EC", "EE", "EH", "ES", "ET", "FI", "FJ", "FK", "FM", "FO", "FR", "GA", "GB", "GD", "GE", "GF", "GG", "GH", "GI", "GL", "GM", "GP", "GQ", "GR", "GS", "GT", "GU", "HK", "HM", "HN", "HR", "HU", "ID", "IE", "IL", "IM", "IN", "IO", "IS", "IT", "JE", "JM", "JO", "JP", "KE", "KG", "KH", "KI", "KM", "KN", "KR", "KW", "KY", "KZ", "LC", "LI", "LK", "LS", "LT", "LU", "LV", "MA", "MC", "ME", "MF", "MG", "MH", "MK", "ML", "MN", "MO", "MP", "MQ", "MR", "MS", "MT", "MU", "MV", "MW", "MX", "MY", "MZ", "NA", "NC", "NE", "NF", "NG", "NI", "NL", "NO", "NP", "NR", "NU", "NZ", "OM", "PA", "PE", "PF", "PH", "PK", "PL", "PM", "PN", "PR", "PS", "PT", "PW", "PY", "QA", "RE", "RO", "RW", "SA", "SB", "SC", "SE", "SG", "SH", "SI", "SJ", "SK", "SL", "SM", "SN", "SR", "ST", "SV", "SX", "SZ", "TC", "TD", "TF", "TG", "TH", "TJ", "TK", "TL", "TM", "TO", "TR", "TT", "TV", "TW", "TZ", "UM", "US", "UY", "UZ", "VA", "VC", "VE", "VG", "VI", "VN", "WF", "WS", "YT", "ZA", "ZM"]
			}`,
			{
				"headers": {
					"Authorization": "Basic stuffz",
					"User-Agent": "SysTest - perf",
					"Accept": "application/json; charset=utf-8",
					"Content-Type": "application/json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 201": (r) => r.status === 201 })) { return };
		redirectUrl = res.headers.Location;
		json = JSON.parse(res.body);
		// Request #1
		res = http.get(redirectUrl,
			{
				"headers": {
					"Authorization": "Basic stuffz",
					"User-Agent": "SysTest - perf",
					"Accept": "application/json; charset=utf-8",
					"Content-Type": "application/json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 200": (r) => r.status === 200 })) { return };
		json = JSON.parse(res.body);
		// Request #2
		res = http.get("https://some-other-host.example.com/yaco/orders/570714bf-3c2b-452e-90cd-f7c5e552bb25",
			{
				"headers": {
					"Authorization": "Checkout otherStuffz",
					"User-Agent": "SysTest - perf",
					"Accept": "application/vnd.checkout.server-order-v1+json",
					"Content-Type": "application/vnd.checkout.client-order-v1+json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-other-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 200": (r) => r.status === 200 })) { return };
		json = JSON.parse(res.body);
		// Request #3
		res = http.post("https://some-other-host.example.com/yaco/orders/570714bf-3c2b-452e-90cd-f7c5e552bb25",
			`{
				"allowed_billing_countries": ["${json.allowed_billing_countries[0]}", "${json.allowed_billing_countries[1]}", "${json.allowed_billing_countries[2]}", "${json.allowed_billing_countries[3]}", "${json.allowed_billing_countries[4]}", "${json.allowed_billing_countries[5]}", "${json.allowed_billing_countries[6]}", "${json.allowed_billing_countries[7]}", "${json.allowed_billing_countries[8]}", "${json.allowed_billing_countries[9]}", "${json.allowed_billing_countries[10]}", "${json.allowed_billing_countries[11]}", "${json.allowed_billing_countries[12]}", "${json.allowed_billing_countries[13]}", "${json.allowed_billing_countries[14]}", "${json.allowed_billing_countries[15]}", "${json.allowed_billing_countries[16]}", "${json.allowed_billing_countries[17]}", "${json.allowed_billing_countries[18]}", "${json.allowed_billing_countries[19]}", "${json.allowed_billing_countries[20]}", "${json.allowed_billing_countries[21]}", "${json.allowed_billing_countries[22]}", "${json.allowed_billing_countries[23]}", "${json.allowed_billing_countries[24]}", "${json.allowed_billing_countries[25]}", "${json.allowed_billing_countries[26]}", "${json.allowed_billing_countries[27]}", "${json.allowed_billing_countries[28]}", "${json.allowed_billing_countries[29]}", "${json.allowed_billing_countries[30]}", "${json.allowed_billing_countries[31]}", "${json.allowed_billing_countries[32]}", "${json.allowed_billing_countries[33]}", "${json.allowed_billing_countries[34]}", "${json.allowed_billing_countries[35]}", "${json.allowed_billing_countries[36]}", "${json.allowed_billing_countries[37]}", "${json.allowed_billing_countries[38]}", "${json.allowed_billing_countries[39]}", "${json.allowed_billing_countries[40]}", "${json.allowed_billing_countries[41]}", "${json.allowed_billing_countries[42]}", "${json.allowed_billing_countries[43]}", "${json.allowed_billing_countries[44]}", "${json.allowed_billing_countries[45]}", "${json.allowed_billing_countries[46]}", "${json.allowed_billing_countries[47]}", "${json.allowed_billing_countries[48]}", "${json.allowed_billing_countries[49]}", "${json.allowed_billing_countries[50]}", "${json.allowed_billing_countries[51]}", "${json.allowed_billing_countries[52]}", "${json.allowed_billing_countries[53]}", "${json.allowed_billing_countries[54]}", "${json.allowed_billing_countries[55]}", "${json.allowed_billing_countries[56]}", "${json.allowed_billing_countries[57]}", "${json.allowed_billing_countries[58]}", "${json.allowed_billing_countries[59]}", "${json.allowed_billing_countries[60]}", "${json.allowed_billing_countries[61]}", "${json.allowed_billing_countries[62]}", "${json.allowed_billing_countries[63]}", "${json.allowed_billing_countries[64]}", "${json.allowed_billing_countries[65]}", "${json.allowed_billing_countries[66]}", "${json.allowed_billing_countries[67]}", "${json.allowed_billing_countries[68]}", "${json.allowed_billing_countries[69]}", "${json.allowed_billing_countries[70]}", "${json.allowed_billing_countries[71]}", "${json.allowed_billing_countries[72]}", "${json.allowed_billing_countries[73]}", "${json.allowed_billing_countries[74]}", "${json.allowed_billing_countries[75]}", "${json.allowed_billing_countries[76]}", "${json.allowed_billing_countries[77]}", "${json.allowed_billing_countries[78]}", "${json.allowed_billing_countries[79]}", "${json.allowed_billing_countries[80]}", "${json.allowed_billing_countries[81]}", "${json.allowed_billing_countries[82]}", "${json.allowed_billing_countries[83]}", "${json.allowed_billing_countries[84]}", "${json.allowed_billing_countries[85]}", "${json.allowed_billing_countries[86]}", "${json.allowed_billing_countries[87]}", "${json.allowed_billing_countries[88]}", "${json.allowed_billing_countries[89]}", "${json.allowed_billing_countries[90]}", "${json.allowed_billing_countries[91]}", "${json.allowed_billing_countries[92]}", "${json.allowed_billing_countries[93]}", "${json.allowed_billing_countries[94]}", "${json.allowed_billing_countries[95]}", "${json.allowed_billing_countries[96]}", "${json.allowed_billing_countries[97]}", "${json.allowed_billing_countries[98]}", "${json.allowed_billing_countries[99]}", "${json.allowed_billing_countries[100]}", "${json.allowed_billing_countries[101]}", "${json.allowed_billing_countries[102]}", "${json.allowed_billing_countries[103]}", "${json.allowed_billing_countries[104]}", "${json.allowed_billing_countries[105]}", "${json.allowed_billing_countries[106]}", "${json.allowed_billing_countries[107]}", "${json.allowed_billing_countries[108]}", "${json.allowed_billing_countries[109]}", "${json.allowed_billing_countries[110]}", "${json.allowed_billing_countries[111]}", "${json.allowed_billing_countries[112]}", "${json.allowed_billing_countries[113]}", "${json.allowed_billing_countries[114]}", "${json.allowed_billing_countries[115]}", "${json.allowed_billing_countries[116]}", "${json.allowed_billing_countries[117]}", "${json.allowed_billing_countries[118]}", "${json.allowed_billing_countries[119]}", "${json.allowed_billing_countries[120]}", "${json.allowed_billing_countries[121]}", "${json.allowed_billing_countries[122]}", "${json.allowed_billing_countries[123]}", "${json.allowed_billing_countries[124]}", "${json.allowed_billing_countries[125]}", "${json.allowed_billing_countries[126]}", "${json.allowed_billing_countries[127]}", "${json.allowed_billing_countries[128]}", "${json.allowed_billing_countries[129]}", "${json.allowed_billing_countries[130]}", "${json.allowed_billing_countries[131]}", "${json.allowed_billing_countries[132]}", "${json.allowed_billing_countries[133]}", "${json.allowed_billing_countries[134]}", "${json.allowed_billing_countries[135]}", "${json.allowed_billing_countries[136]}", "${json.allowed_billing_countries[137]}", "${json.allowed_billing_countries[138]}", "${json.allowed_billing_countries[139]}", "${json.allowed_billing_countries[140]}", "${json.allowed_billing_countries[141]}", "${json.allowed_billing_countries[142]}", "${json.allowed_billing_countries[143]}", "${json.allowed_billing_countries[144]}", "${json.allowed_billing_countries[145]}", "${json.allowed_billing_countries[146]}", "${json.allowed_billing_countries[147]}", "${json.allowed_billing_countries[148]}", "${json.allowed_billing_countries[149]}", "${json.allowed_billing_countries[150]}", "${json.allowed_billing_countries[151]}", "${json.allowed_billing_countries[152]}", "${json.allowed_billing_countries[153]}", "${json.allowed_billing_countries[154]}", "${json.allowed_billing_countries[155]}", "${json.allowed_billing_countries[156]}", "${json.allowed_billing_countries[157]}", "${json.allowed_billing_countries[158]}", "${json.allowed_billing_countries[159]}", "${json.allowed_billing_countries[160]}", "${json.allowed_billing_countries[161]}", "${json.allowed_billing_countries[162]}", "${json.allowed_billing_countries[163]}", "${json.allowed_billing_countries[164]}", "${json.allowed_billing_countries[165]}", "${json.allowed_billing_countries[166]}", "${json.allowed_billing_countries[167]}", "${json.allowed_billing_countries[168]}", "${json.allowed_billing_countries[169]}", "${json.allowed_billing_countries[170]}", "${json.allowed_billing_countries[171]}", "${json.allowed_billing_countries[172]}", "${json.allowed_billing_countries[173]}", "${json.allowed_billing_countries[174]}", "${json.allowed_billing_countries[175]}", "${json.allowed_billing_countries[176]}", "${json.allowed_billing_countries[177]}", "${json.allowed_billing_countries[178]}", "${json.allowed_billing_countries[179]}", "${json.allowed_billing_countries[180]}", "${json.allowed_billing_countries[181]}", "${json.allowed_billing_countries[182]}", "${json.allowed_billing_countries[183]}", "${json.allowed_billing_countries[184]}", "${json.allowed_billing_countries[185]}", "${json.allowed_billing_countries[186]}", "${json.allowed_billing_countries[187]}", "${json.allowed_billing_countries[188]}", "${json.allowed_billing_countries[189]}", "${json.allowed_billing_countries[190]}", "${json.allowed_billing_countries[191]}", "${json.allowed_billing_countries[192]}", "${json.allowed_billing_countries[193]}", "${json.allowed_billing_countries[194]}", "${json.allowed_billing_countries[195]}", "${json.allowed_billing_countries[196]}", "${json.allowed_billing_countries[197]}", "${json.allowed_billing_countries[198]}", "${json.allowed_billing_countries[199]}", "${json.allowed_billing_countries[200]}", "${json.allowed_billing_countries[201]}", "${json.allowed_billing_countries[202]}", "${json.allowed_billing_countries[203]}", "${json.allowed_billing_countries[204]}", "${json.allowed_billing_countries[205]}", "${json.allowed_billing_countries[206]}", "${json.allowed_billing_countries[207]}", "${json.allowed_billing_countries[208]}", "${json.allowed_billing_countries[209]}", "${json.allowed_billing_countries[210]}"],
				"allowed_shipping_countries": ["${json.allowed_shipping_countries[0]}", "${json.allowed_shipping_countries[1]}", "${json.allowed_shipping_countries[2]}", "${json.allowed_shipping_countries[3]}", "${json.allowed_shipping_countries[4]}", "${json.allowed_shipping_countries[5]}", "${json.allowed_shipping_countries[6]}", "${json.allowed_shipping_countries[7]}", "${json.allowed_shipping_countries[8]}", "${json.allowed_shipping_countries[9]}", "${json.allowed_shipping_countries[10]}", "${json.allowed_shipping_countries[11]}", "${json.allowed_shipping_countries[12]}", "${json.allowed_shipping_countries[13]}", "${json.allowed_shipping_countries[14]}", "${json.allowed_shipping_countries[15]}", "${json.allowed_shipping_countries[16]}", "${json.allowed_shipping_countries[17]}", "${json.allowed_shipping_countries[18]}", "${json.allowed_shipping_countries[19]}", "${json.allowed_shipping_countries[20]}", "${json.allowed_shipping_countries[21]}", "${json.allowed_shipping_countries[22]}", "${json.allowed_shipping_countries[23]}", "${json.allowed_shipping_countries[24]}", "${json.allowed_shipping_countries[25]}", "${json.allowed_shipping_countries[26]}", "${json.allowed_shipping_countries[27]}", "${json.allowed_shipping_countries[28]}", "${json.allowed_shipping_countries[29]}", "${json.allowed_shipping_countries[30]}", "${json.allowed_shipping_countries[31]}", "${json.allowed_shipping_countries[32]}", "${json.allowed_shipping_countries[33]}", "${json.allowed_shipping_countries[34]}", "${json.allowed_shipping_countries[35]}", "${json.allowed_shipping_countries[36]}", "${json.allowed_shipping_countries[37]}", "${json.allowed_shipping_countries[38]}", "${json.allowed_shipping_countries[39]}", "${json.allowed_shipping_countries[40]}", "${json.allowed_shipping_countries[41]}", "${json.allowed_shipping_countries[42]}", "${json.allowed_shipping_countries[43]}", "${json.allowed_shipping_countries[44]}", "${json.allowed_shipping_countries[45]}", "${json.allowed_shipping_countries[46]}", "${json.allowed_shipping_countries[47]}", "${json.allowed_shipping_countries[48]}", "${json.allowed_shipping_countries[49]}", "${json.allowed_shipping_countries[50]}", "${json.allowed_shipping_countries[51]}", "${json.allowed_shipping_countries[52]}", "${json.allowed_shipping_countries[53]}", "${json.allowed_shipping_countries[54]}", "${json.allowed_shipping_countries[55]}", "${json.allowed_shipping_countries[56]}", "${json.allowed_shipping_countries[57]}", "${json.allowed_shipping_countries[58]}", "${json.allowed_shipping_countries[59]}", "${json.allowed_shipping_countries[60]}", "${json.allowed_shipping_countries[61]}", "${json.allowed_shipping_countries[62]}", "${json.allowed_shipping_countries[63]}", "${json.allowed_shipping_countries[64]}", "${json.allowed_shipping_countries[65]}", "${json.allowed_shipping_countries[66]}", "${json.allowed_shipping_countries[67]}", "${json.allowed_shipping_countries[68]}", "${json.allowed_shipping_countries[69]}", "${json.allowed_shipping_countries[70]}", "${json.allowed_shipping_countries[71]}", "${json.allowed_shipping_countries[72]}", "${json.allowed_shipping_countries[73]}", "${json.allowed_shipping_countries[74]}", "${json.allowed_shipping_countries[75]}", "${json.allowed_shipping_countries[76]}", "${json.allowed_shipping_countries[77]}", "${json.allowed_shipping_countries[78]}", "${json.allowed_shipping_countries[79]}", "${json.allowed_shipping_countries[80]}", "${json.allowed_shipping_countries[81]}", "${json.allowed_shipping_countries[82]}", "${json.allowed_shipping_countries[83]}", "${json.allowed_shipping_countries[84]}", "${json.allowed_shipping_countries[85]}", "${json.allowed_shipping_countries[86]}", "${json.allowed_shipping_countries[87]}", "${json.allowed_shipping_countries[88]}", "${json.allowed_shipping_countries[89]}", "${json.allowed_shipping_countries[90]}", "${json.allowed_shipping_countries[91]}", "${json.allowed_shipping_countries[92]}", "${json.allowed_shipping_countries[93]}", "${json.allowed_shipping_countries[94]}", "${json.allowed_shipping_countries[95]}", "${json.allowed_shipping_countries[96]}", "${json.allowed_shipping_countries[97]}", "${json.allowed_shipping_countries[98]}", "${json.allowed_shipping_countries[99]}", "${json.allowed_shipping_countries[100]}", "${json.allowed_shipping_countries[101]}", "${json.allowed_shipping_countries[102]}", "${json.allowed_shipping_countries[103]}", "${json.allowed_shipping_countries[104]}", "${json.allowed_shipping_countries[105]}", "${json.allowed_shipping_countries[106]}", "${json.allowed_shipping_countries[107]}", "${json.allowed_shipping_countries[108]}", "${json.allowed_shipping_countries[109]}", "${json.allowed_shipping_countries[110]}", "${json.allowed_shipping_countries[111]}", "${json.allowed_shipping_countries[112]}", "${json.allowed_shipping_countries[113]}", "${json.allowed_shipping_countries[114]}", "${json.allowed_shipping_countries[115]}", "${json.allowed_shipping_countries[116]}", "${json.allowed_shipping_countries[117]}", "${json.allowed_shipping_countries[118]}", "${json.allowed_shipping_countries[119]}", "${json.allowed_shipping_countries[120]}", "${json.allowed_shipping_countries[121]}", "${json.allowed_shipping_countries[122]}", "${json.allowed_shipping_countries[123]}", "${json.allowed_shipping_countries[124]}", "${json.allowed_shipping_countries[125]}", "${json.allowed_shipping_countries[126]}", "${json.allowed_shipping_countries[127]}", "${json.allowed_shipping_countries[128]}", "${json.allowed_shipping_countries[129]}", "${json.allowed_shipping_countries[130]}", "${json.allowed_shipping_countries[131]}", "${json.allowed_shipping_countries[132]}", "${json.allowed_shipping_countries[133]}", "${json.allowed_shipping_countries[134]}", "${json.allowed_shipping_countries[135]}", "${json.allowed_shipping_countries[136]}", "${json.allowed_shipping_countries[137]}", "${json.allowed_shipping_countries[138]}", "${json.allowed_shipping_countries[139]}", "${json.allowed_shipping_countries[140]}", "${json.allowed_shipping_countries[141]}", "${json.allowed_shipping_countries[142]}", "${json.allowed_shipping_countries[143]}", "${json.allowed_shipping_countries[144]}", "${json.allowed_shipping_countries[145]}", "${json.allowed_shipping_countries[146]}", "${json.allowed_shipping_countries[147]}", "${json.allowed_shipping_countries[148]}", "${json.allowed_shipping_countries[149]}", "${json.allowed_shipping_countries[150]}", "${json.allowed_shipping_countries[151]}", "${json.allowed_shipping_countries[152]}", "${json.allowed_shipping_countries[153]}", "${json.allowed_shipping_countries[154]}", "${json.allowed_shipping_countries[155]}", "${json.allowed_shipping_countries[156]}", "${json.allowed_shipping_countries[157]}", "${json.allowed_shipping_countries[158]}", "${json.allowed_shipping_countries[159]}", "${json.allowed_shipping_countries[160]}", "${json.allowed_shipping_countries[161]}", "${json.allowed_shipping_countries[162]}", "${json.allowed_shipping_countries[163]}", "${json.allowed_shipping_countries[164]}", "${json.allowed_shipping_countries[165]}", "${json.allowed_shipping_countries[166]}", "${json.allowed_shipping_countries[167]}", "${json.allowed_shipping_countries[168]}", "${json.allowed_shipping_countries[169]}", "${json.allowed_shipping_countries[170]}", "${json.allowed_shipping_countries[171]}", "${json.allowed_shipping_countries[172]}", "${json.allowed_shipping_countries[173]}", "${json.allowed_shipping_countries[174]}", "${json.allowed_shipping_countries[175]}", "${json.allowed_shipping_countries[176]}", "${json.allowed_shipping_countries[177]}", "${json.allowed_shipping_countries[178]}", "${json.allowed_shipping_countries[179]}", "${json.allowed_shipping_countries[180]}", "${json.allowed_shipping_countries[181]}", "${json.allowed_shipping_countries[182]}", "${json.allowed_shipping_countries[183]}", "${json.allowed_shipping_countries[184]}", "${json.allowed_shipping_countries[185]}", "${json.allowed_shipping_countries[186]}", "${json.allowed_shipping_countries[187]}", "${json.allowed_shipping_countries[188]}", "${json.allowed_shipping_countries[189]}", "${json.allowed_shipping_countries[190]}", "${json.allowed_shipping_countries[191]}", "${json.allowed_shipping_countries[192]}", "${json.allowed_shipping_countries[193]}", "${json.allowed_shipping_countries[194]}", "${json.allowed_shipping_countries[195]}", "${json.allowed_shipping_countries[196]}", "${json.allowed_shipping_countries[197]}", "${json.allowed_shipping_countries[198]}", "${json.allowed_shipping_countries[199]}", "${json.allowed_shipping_countries[200]}", "${json.allowed_shipping_countries[201]}", "${json.allowed_shipping_countries[202]}", "${json.allowed_shipping_countries[203]}", "${json.allowed_shipping_countries[204]}", "${json.allowed_shipping_countries[205]}", "${json.allowed_shipping_countries[206]}", "${json.allowed_shipping_countries[207]}", "${json.allowed_shipping_countries[208]}", "${json.allowed_shipping_countries[209]}", "${json.allowed_shipping_countries[210]}"],
				"cart": {
					"items": [
						{
							"image_url": "${json.cart.items[0].image_url}",
							"name": "${json.cart.items[0].name}",
							"product_url": "${json.cart.items[0].product_url}",
							"quantity": "${json.cart.items[0].quantity}",
							"reference": "${json.cart.items[0].reference}",
							"tax_rate": "${json.cart.items[0].tax_rate}",
							"total_price_excluding_tax": "${json.cart.items[0].total_price_excluding_tax}",
							"total_price_including_tax": "${json.cart.items[0].total_price_including_tax}",
							"total_tax_amount": "${json.cart.items[0].total_tax_amount}",
							"type": "${json.cart.items[0].type}",
							"unit_price": "${json.cart.items[0].unit_price}"
						}
					],
					"subtotal": "${json.cart.subtotal}",
					"total_discount_amount_excluding_tax": "${json.cart.total_discount_amount_excluding_tax}",
					"total_price_excluding_tax": "${json.cart.total_price_excluding_tax}",
					"total_price_including_tax": "${json.cart.total_price_including_tax}",
					"total_shipping_amount_excluding_tax": "${json.cart.total_shipping_amount_excluding_tax}",
					"total_store_credit": "${json.cart.total_store_credit}",
					"total_surcharge_amount_excluding_tax": "${json.cart.total_surcharge_amount_excluding_tax}",
					"total_tax_amount": "${json.cart.total_tax_amount}"
				},
				"merchant_urls": {
					"checkout": "${json.merchant_urls.checkout}",
					"confirmation": "${json.merchant_urls.confirmation}",
					"terms": "${json.merchant_urls.terms}"
				},
				"options": {
					"allow_separate_shipping_address": "${json.options.allow_separate_shipping_address}",
					"allowed_customer_types": ["${json.options.allowed_customer_types[0]}"],
					"date_of_birth_mandatory": "${json.options.date_of_birth_mandatory}",
					"national_identification_number_mandatory": "${json.options.national_identification_number_mandatory}",
					"payment_selector_on_load": "${json.options.payment_selector_on_load}"
				},
				"preview_payment_methods": [
					{
						"data": {
							"days": "${json.preview_payment_methods[0].data.days}"
						},
						"id": "${json.preview_payment_methods[0].id}",
						"type": "${json.preview_payment_methods[0].type}"
					},
					{
						"data": {
							"allow_saved_card": "${json.preview_payment_methods[1].data.allow_saved_card}",
							"available_cards": ["${json.preview_payment_methods[1].data.available_cards[0]}", "${json.preview_payment_methods[1].data.available_cards[1]}"],
							"do_save_card": "${json.preview_payment_methods[1].data.do_save_card}"
						},
						"id": "${json.preview_payment_methods[1].id}",
						"type": "${json.preview_payment_methods[1].type}"
					}
				],
				"required_fields": ["${json.required_fields[0]}", "${json.required_fields[1]}"],
				"shared": {
					"billing_address": {
						"country": "${json.shared.billing_address.country}"
					},
					"challenge": {
						"country": "${json.shared.challenge.country}",
						"email": "drop+b28643c0e7c74da6b6ff2f4131aa3d64+d0+gr@example.com",
						"postal_code": "10066"
					},
					"currency": "${json.shared.currency}",
					"customer": {
						"type": "${json.shared.customer.type}"
					},
					"language": "${json.shared.language}"
				},
				"status": {
					"prescreened": "${json.status.prescreened}",
					"require_terms_consent": "${json.status.require_terms_consent}"
				}
			}`,
			{
				"headers": {
					"Authorization": "Checkout otherStuffz",
					"User-Agent": "SysTest - perf",
					"Content-Type": "application/vnd.checkout.client-order-v1+json",
					"Accept": "application/vnd.checkout.server-order-v1+json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-other-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 200": (r) => r.status === 200 })) { return };
		json = JSON.parse(res.body);
		// Request #4
		res = http.post("https://some-other-host.example.com/yaco/orders/570714bf-3c2b-452e-90cd-f7c5e552bb25",
			`{
				"allowed_billing_countries": ["${json.allowed_billing_countries[0]}", "${json.allowed_billing_countries[1]}", "${json.allowed_billing_countries[2]}", "${json.allowed_billing_countries[3]}", "${json.allowed_billing_countries[4]}", "${json.allowed_billing_countries[5]}", "${json.allowed_billing_countries[6]}", "${json.allowed_billing_countries[7]}", "${json.allowed_billing_countries[8]}", "${json.allowed_billing_countries[9]}", "${json.allowed_billing_countries[10]}", "${json.allowed_billing_countries[11]}", "${json.allowed_billing_countries[12]}", "${json.allowed_billing_countries[13]}", "${json.allowed_billing_countries[14]}", "${json.allowed_billing_countries[15]}", "${json.allowed_billing_countries[16]}", "${json.allowed_billing_countries[17]}", "${json.allowed_billing_countries[18]}", "${json.allowed_billing_countries[19]}", "${json.allowed_billing_countries[20]}", "${json.allowed_billing_countries[21]}", "${json.allowed_billing_countries[22]}", "${json.allowed_billing_countries[23]}", "${json.allowed_billing_countries[24]}", "${json.allowed_billing_countries[25]}", "${json.allowed_billing_countries[26]}", "${json.allowed_billing_countries[27]}", "${json.allowed_billing_countries[28]}", "${json.allowed_billing_countries[29]}", "${json.allowed_billing_countries[30]}", "${json.allowed_billing_countries[31]}", "${json.allowed_billing_countries[32]}", "${json.allowed_billing_countries[33]}", "${json.allowed_billing_countries[34]}", "${json.allowed_billing_countries[35]}", "${json.allowed_billing_countries[36]}", "${json.allowed_billing_countries[37]}", "${json.allowed_billing_countries[38]}", "${json.allowed_billing_countries[39]}", "${json.allowed_billing_countries[40]}", "${json.allowed_billing_countries[41]}", "${json.allowed_billing_countries[42]}", "${json.allowed_billing_countries[43]}", "${json.allowed_billing_countries[44]}", "${json.allowed_billing_countries[45]}", "${json.allowed_billing_countries[46]}", "${json.allowed_billing_countries[47]}", "${json.allowed_billing_countries[48]}", "${json.allowed_billing_countries[49]}", "${json.allowed_billing_countries[50]}", "${json.allowed_billing_countries[51]}", "${json.allowed_billing_countries[52]}", "${json.allowed_billing_countries[53]}", "${json.allowed_billing_countries[54]}", "${json.allowed_billing_countries[55]}", "${json.allowed_billing_countries[56]}", "${json.allowed_billing_countries[57]}", "${json.allowed_billing_countries[58]}", "${json.allowed_billing_countries[59]}", "${json.allowed_billing_countries[60]}", "${json.allowed_billing_countries[61]}", "${json.allowed_billing_countries[62]}", "${json.allowed_billing_countries[63]}", "${json.allowed_billing_countries[64]}", "${json.allowed_billing_countries[65]}", "${json.allowed_billing_countries[66]}", "${json.allowed_billing_countries[67]}", "${json.allowed_billing_countries[68]}", "${json.allowed_billing_countries[69]}", "${json.allowed_billing_countries[70]}", "${json.allowed_billing_countries[71]}", "${json.allowed_billing_countries[72]}", "${json.allowed_billing_countries[73]}", "${json.allowed_billing_countries[74]}", "${json.allowed_billing_countries[75]}", "${json.allowed_billing_countries[76]}", "${json.allowed_billing_countries[77]}", "${json.allowed_billing_countries[78]}", "${json.allowed_billing_countries[79]}", "${json.allowed_billing_countries[80]}", "${json.allowed_billing_countries[81]}", "${json.allowed_billing_countries[82]}", "${json.allowed_billing_countries[83]}", "${json.allowed_billing_countries[84]}", "${json.allowed_billing_countries[85]}", "${json.allowed_billing_countries[86]}", "${json.allowed_billing_countries[87]}", "${json.allowed_billing_countries[88]}", "${json.allowed_billing_countries[89]}", "${json.allowed_billing_countries[90]}", "${json.allowed_billing_countries[91]}", "${json.allowed_billing_countries[92]}", "${json.allowed_billing_countries[93]}", "${json.allowed_billing_countries[94]}", "${json.allowed_billing_countries[95]}", "${json.allowed_billing_countries[96]}", "${json.allowed_billing_countries[97]}", "${json.allowed_billing_countries[98]}", "${json.allowed_billing_countries[99]}", "${json.allowed_billing_countries[100]}", "${json.allowed_billing_countries[101]}", "${json.allowed_billing_countries[102]}", "${json.allowed_billing_countries[103]}", "${json.allowed_billing_countries[104]}", "${json.allowed_billing_countries[105]}", "${json.allowed_billing_countries[106]}", "${json.allowed_billing_countries[107]}", "${json.allowed_billing_countries[108]}", "${json.allowed_billing_countries[109]}", "${json.allowed_billing_countries[110]}", "${json.allowed_billing_countries[111]}", "${json.allowed_billing_countries[112]}", "${json.allowed_billing_countries[113]}", "${json.allowed_billing_countries[114]}", "${json.allowed_billing_countries[115]}", "${json.allowed_billing_countries[116]}", "${json.allowed_billing_countries[117]}", "${json.allowed_billing_countries[118]}", "${json.allowed_billing_countries[119]}", "${json.allowed_billing_countries[120]}", "${json.allowed_billing_countries[121]}", "${json.allowed_billing_countries[122]}", "${json.allowed_billing_countries[123]}", "${json.allowed_billing_countries[124]}", "${json.allowed_billing_countries[125]}", "${json.allowed_billing_countries[126]}", "${json.allowed_billing_countries[127]}", "${json.allowed_billing_countries[128]}", "${json.allowed_billing_countries[129]}", "${json.allowed_billing_countries[130]}", "${json.allowed_billing_countries[131]}", "${json.allowed_billing_countries[132]}", "${json.allowed_billing_countries[133]}", "${json.allowed_billing_countries[134]}", "${json.allowed_billing_countries[135]}", "${json.allowed_billing_countries[136]}", "${json.allowed_billing_countries[137]}", "${json.allowed_billing_countries[138]}", "${json.allowed_billing_countries[139]}", "${json.allowed_billing_countries[140]}", "${json.allowed_billing_countries[141]}", "${json.allowed_billing_countries[142]}", "${json.allowed_billing_countries[143]}", "${json.allowed_billing_countries[144]}", "${json.allowed_billing_countries[145]}", "${json.allowed_billing_countries[146]}", "${json.allowed_billing_countries[147]}", "${json.allowed_billing_countries[148]}", "${json.allowed_billing_countries[149]}", "${json.allowed_billing_countries[150]}", "${json.allowed_billing_countries[151]}", "${json.allowed_billing_countries[152]}", "${json.allowed_billing_countries[153]}", "${json.allowed_billing_countries[154]}", "${json.allowed_billing_countries[155]}", "${json.allowed_billing_countries[156]}", "${json.allowed_billing_countries[157]}", "${json.allowed_billing_countries[158]}", "${json.allowed_billing_countries[159]}", "${json.allowed_billing_countries[160]}", "${json.allowed_billing_countries[161]}", "${json.allowed_billing_countries[162]}", "${json.allowed_billing_countries[163]}", "${json.allowed_billing_countries[164]}", "${json.allowed_billing_countries[165]}", "${json.allowed_billing_countries[166]}", "${json.allowed_billing_countries[167]}", "${json.allowed_billing_countries[168]}", "${json.allowed_billing_countries[169]}", "${json.allowed_billing_countries[170]}", "${json.allowed_billing_countries[171]}", "${json.allowed_billing_countries[172]}", "${json.allowed_billing_countries[173]}", "${json.allowed_billing_countries[174]}", "${json.allowed_billing_countries[175]}", "${json.allowed_billing_countries[176]}", "${json.allowed_billing_countries[177]}", "${json.allowed_billing_countries[178]}", "${json.allowed_billing_countries[179]}", "${json.allowed_billing_countries[180]}", "${json.allowed_billing_countries[181]}", "${json.allowed_billing_countries[182]}", "${json.allowed_billing_countries[183]}", "${json.allowed_billing_countries[184]}", "${json.allowed_billing_countries[185]}", "${json.allowed_billing_countries[186]}", "${json.allowed_billing_countries[187]}", "${json.allowed_billing_countries[188]}", "${json.allowed_billing_countries[189]}", "${json.allowed_billing_countries[190]}", "${json.allowed_billing_countries[191]}", "${json.allowed_billing_countries[192]}", "${json.allowed_billing_countries[193]}", "${json.allowed_billing_countries[194]}", "${json.allowed_billing_countries[195]}", "${json.allowed_billing_countries[196]}", "${json.allowed_billing_countries[197]}", "${json.allowed_billing_countries[198]}", "${json.allowed_billing_countries[199]}", "${json.allowed_billing_countries[200]}", "${json.allowed_billing_countries[201]}", "${json.allowed_billing_countries[202]}", "${json.allowed_billing_countries[203]}", "${json.allowed_billing_countries[204]}", "${json.allowed_billing_countries[205]}", "${json.allowed_billing_countries[206]}", "${json.allowed_billing_countries[207]}", "${json.allowed_billing_countries[208]}", "${json.allowed_billing_countries[209]}", "${json.allowed_billing_countries[210]}"],
				"allowed_shipping_countries": ["${json.allowed_shipping_countries[0]}", "${json.allowed_shipping_countries[1]}", "${json.allowed_shipping_countries[2]}", "${json.allowed_shipping_countries[3]}", "${json.allowed_shipping_countries[4]}", "${json.allowed_shipping_countries[5]}", "${json.allowed_shipping_countries[6]}", "${json.allowed_shipping_countries[7]}", "${json.allowed_shipping_countries[8]}", "${json.allowed_shipping_countries[9]}", "${json.allowed_shipping_countries[10]}", "${json.allowed_shipping_countries[11]}", "${json.allowed_shipping_countries[12]}", "${json.allowed_shipping_countries[13]}", "${json.allowed_shipping_countries[14]}", "${json.allowed_shipping_countries[15]}", "${json.allowed_shipping_countries[16]}", "${json.allowed_shipping_countries[17]}", "${json.allowed_shipping_countries[18]}", "${json.allowed_shipping_countries[19]}", "${json.allowed_shipping_countries[20]}", "${json.allowed_shipping_countries[21]}", "${json.allowed_shipping_countries[22]}", "${json.allowed_shipping_countries[23]}", "${json.allowed_shipping_countries[24]}", "${json.allowed_shipping_countries[25]}", "${json.allowed_shipping_countries[26]}", "${json.allowed_shipping_countries[27]}", "${json.allowed_shipping_countries[28]}", "${json.allowed_shipping_countries[29]}", "${json.allowed_shipping_countries[30]}", "${json.allowed_shipping_countries[31]}", "${json.allowed_shipping_countries[32]}", "${json.allowed_shipping_countries[33]}", "${json.allowed_shipping_countries[34]}", "${json.allowed_shipping_countries[35]}", "${json.allowed_shipping_countries[36]}", "${json.allowed_shipping_countries[37]}", "${json.allowed_shipping_countries[38]}", "${json.allowed_shipping_countries[39]}", "${json.allowed_shipping_countries[40]}", "${json.allowed_shipping_countries[41]}", "${json.allowed_shipping_countries[42]}", "${json.allowed_shipping_countries[43]}", "${json.allowed_shipping_countries[44]}", "${json.allowed_shipping_countries[45]}", "${json.allowed_shipping_countries[46]}", "${json.allowed_shipping_countries[47]}", "${json.allowed_shipping_countries[48]}", "${json.allowed_shipping_countries[49]}", "${json.allowed_shipping_countries[50]}", "${json.allowed_shipping_countries[51]}", "${json.allowed_shipping_countries[52]}", "${json.allowed_shipping_countries[53]}", "${json.allowed_shipping_countries[54]}", "${json.allowed_shipping_countries[55]}", "${json.allowed_shipping_countries[56]}", "${json.allowed_shipping_countries[57]}", "${json.allowed_shipping_countries[58]}", "${json.allowed_shipping_countries[59]}", "${json.allowed_shipping_countries[60]}", "${json.allowed_shipping_countries[61]}", "${json.allowed_shipping_countries[62]}", "${json.allowed_shipping_countries[63]}", "${json.allowed_shipping_countries[64]}", "${json.allowed_shipping_countries[65]}", "${json.allowed_shipping_countries[66]}", "${json.allowed_shipping_countries[67]}", "${json.allowed_shipping_countries[68]}", "${json.allowed_shipping_countries[69]}", "${json.allowed_shipping_countries[70]}", "${json.allowed_shipping_countries[71]}", "${json.allowed_shipping_countries[72]}", "${json.allowed_shipping_countries[73]}", "${json.allowed_shipping_countries[74]}", "${json.allowed_shipping_countries[75]}", "${json.allowed_shipping_countries[76]}", "${json.allowed_shipping_countries[77]}", "${json.allowed_shipping_countries[78]}", "${json.allowed_shipping_countries[79]}", "${json.allowed_shipping_countries[80]}", "${json.allowed_shipping_countries[81]}", "${json.allowed_shipping_countries[82]}", "${json.allowed_shipping_countries[83]}", "${json.allowed_shipping_countries[84]}", "${json.allowed_shipping_countries[85]}", "${json.allowed_shipping_countries[86]}", "${json.allowed_shipping_countries[87]}", "${json.allowed_shipping_countries[88]}", "${json.allowed_shipping_countries[89]}", "${json.allowed_shipping_countries[90]}", "${json.allowed_shipping_countries[91]}", "${json.allowed_shipping_countries[92]}", "${json.allowed_shipping_countries[93]}", "${json.allowed_shipping_countries[94]}", "${json.allowed_shipping_countries[95]}", "${json.allowed_shipping_countries[96]}", "${json.allowed_shipping_countries[97]}", "${json.allowed_shipping_countries[98]}", "${json.allowed_shipping_countries[99]}", "${json.allowed_shipping_countries[100]}", "${json.allowed_shipping_countries[101]}", "${json.allowed_shipping_countries[102]}", "${json.allowed_shipping_countries[103]}", "${json.allowed_shipping_countries[104]}", "${json.allowed_shipping_countries[105]}", "${json.allowed_shipping_countries[106]}", "${json.allowed_shipping_countries[107]}", "${json.allowed_shipping_countries[108]}", "${json.allowed_shipping_countries[109]}", "${json.allowed_shipping_countries[110]}", "${json.allowed_shipping_countries[111]}", "${json.allowed_shipping_countries[112]}", "${json.allowed_shipping_countries[113]}", "${json.allowed_shipping_countries[114]}", "${json.allowed_shipping_countries[115]}", "${json.allowed_shipping_countries[116]}", "${json.allowed_shipping_countries[117]}", "${json.allowed_shipping_countries[118]}", "${json.allowed_shipping_countries[119]}", "${json.allowed_shipping_countries[120]}", "${json.allowed_shipping_countries[121]}", "${json.allowed_shipping_countries[122]}", "${json.allowed_shipping_countries[123]}", "${json.allowed_shipping_countries[124]}", "${json.allowed_shipping_countries[125]}", "${json.allowed_shipping_countries[126]}", "${json.allowed_shipping_countries[127]}", "${json.allowed_shipping_countries[128]}", "${json.allowed_shipping_countries[129]}", "${json.allowed_shipping_countries[130]}", "${json.allowed_shipping_countries[131]}", "${json.allowed_shipping_countries[132]}", "${json.allowed_shipping_countries[133]}", "${json.allowed_shipping_countries[134]}", "${json.allowed_shipping_countries[135]}", "${json.allowed_shipping_countries[136]}", "${json.allowed_shipping_countries[137]}", "${json.allowed_shipping_countries[138]}", "${json.allowed_shipping_countries[139]}", "${json.allowed_shipping_countries[140]}", "${json.allowed_shipping_countries[141]}", "${json.allowed_shipping_countries[142]}", "${json.allowed_shipping_countries[143]}", "${json.allowed_shipping_countries[144]}", "${json.allowed_shipping_countries[145]}", "${json.allowed_shipping_countries[146]}", "${json.allowed_shipping_countries[147]}", "${json.allowed_shipping_countries[148]}", "${json.allowed_shipping_countries[149]}", "${json.allowed_shipping_countries[150]}", "${json.allowed_shipping_countries[151]}", "${json.allowed_shipping_countries[152]}", "${json.allowed_shipping_countries[153]}", "${json.allowed_shipping_countries[154]}", "${json.allowed_shipping_countries[155]}", "${json.allowed_shipping_countries[156]}", "${json.allowed_shipping_countries[157]}", "${json.allowed_shipping_countries[158]}", "${json.allowed_shipping_countries[159]}", "${json.allowed_shipping_countries[160]}", "${json.allowed_shipping_countries[161]}", "${json.allowed_shipping_countries[162]}", "${json.allowed_shipping_countries[163]}", "${json.allowed_shipping_countries[164]}", "${json.allowed_shipping_countries[165]}", "${json.allowed_shipping_countries[166]}", "${json.allowed_shipping_countries[167]}", "${json.allowed_shipping_countries[168]}", "${json.allowed_shipping_countries[169]}", "${json.allowed_shipping_countries[170]}", "${json.allowed_shipping_countries[171]}", "${json.allowed_shipping_countries[172]}", "${json.allowed_shipping_countries[173]}", "${json.allowed_shipping_countries[174]}", "${json.allowed_shipping_countries[175]}", "${json.allowed_shipping_countries[176]}", "${json.allowed_shipping_countries[177]}", "${json.allowed_shipping_countries[178]}", "${json.allowed_shipping_countries[179]}", "${json.allowed_shipping_countries[180]}", "${json.allowed_shipping_countries[181]}", "${json.allowed_shipping_countries[182]}", "${json.allowed_shipping_countries[183]}", "${json.allowed_shipping_countries[184]}", "${json.allowed_shipping_countries[185]}", "${json.allowed_shipping_countries[186]}", "${json.allowed_shipping_countries[187]}", "${json.allowed_shipping_countries[188]}", "${json.allowed_shipping_countries[189]}", "${json.allowed_shipping_countries[190]}", "${json.allowed_shipping_countries[191]}", "${json.allowed_shipping_countries[192]}", "${json.allowed_shipping_countries[193]}", "${json.allowed_shipping_countries[194]}", "${json.allowed_shipping_countries[195]}", "${json.allowed_shipping_countries[196]}", "${json.allowed_shipping_countries[197]}", "${json.allowed_shipping_countries[198]}", "${json.allowed_shipping_countries[199]}", "${json.allowed_shipping_countries[200]}", "${json.allowed_shipping_countries[201]}", "${json.allowed_shipping_countries[202]}", "${json.allowed_shipping_countries[203]}", "${json.allowed_shipping_countries[204]}", "${json.allowed_shipping_countries[205]}", "${json.allowed_shipping_countries[206]}", "${json.allowed_shipping_countries[207]}", "${json.allowed_shipping_countries[208]}", "${json.allowed_shipping_countries[209]}", "${json.allowed_shipping_countries[210]}"],
				"analytics_user_id": "${json.analytics_user_id}",
				"cart": {
					"items": [
						{
							"image_url": "${json.cart.items[0].image_url}",
							"name": "${json.cart.items[0].name}",
							"product_url": "${json.cart.items[0].product_url}",
							"quantity": "${json.cart.items[0].quantity}",
							"reference": "${json.cart.items[0].reference}",
							"tax_rate": "${json.cart.items[0].tax_rate}",
							"total_price_excluding_tax": "${json.cart.items[0].total_price_excluding_tax}",
							"total_price_including_tax": "${json.cart.items[0].total_price_including_tax}",
							"total_tax_amount": "${json.cart.items[0].total_tax_amount}",
							"type": "${json.cart.items[0].type}",
							"unit_price": "${json.cart.items[0].unit_price}"
						}
					],
					"subtotal": "${json.cart.subtotal}",
					"total_discount_amount_excluding_tax": "${json.cart.total_discount_amount_excluding_tax}",
					"total_price_excluding_tax": "${json.cart.total_price_excluding_tax}",
					"total_price_including_tax": "${json.cart.total_price_including_tax}",
					"total_shipping_amount_excluding_tax": "${json.cart.total_shipping_amount_excluding_tax}",
					"total_store_credit": "${json.cart.total_store_credit}",
					"total_surcharge_amount_excluding_tax": "${json.cart.total_surcharge_amount_excluding_tax}",
					"total_tax_amount": "${json.cart.total_tax_amount}"
				},
				"correlation_id": "f6df29e7-f850-4c36-81fc-11def2f44b81",
				"merchant_urls": {
					"checkout": "${json.merchant_urls.checkout}",
					"confirmation": "${json.merchant_urls.confirmation}",
					"terms": "${json.merchant_urls.terms}"
				},
				"options": {
					"allow_separate_shipping_address": "${json.options.allow_separate_shipping_address}",
					"allowed_customer_types": ["${json.options.allowed_customer_types[0]}"],
					"date_of_birth_mandatory": "${json.options.date_of_birth_mandatory}",
					"national_identification_number_mandatory": "${json.options.national_identification_number_mandatory}",
					"payment_selector_on_load": "${json.options.payment_selector_on_load}"
				},
				"preview_payment_methods": [
					{
						"data": {
							"days": "${json.preview_payment_methods[0].data.days}"
						},
						"id": "${json.preview_payment_methods[0].id}",
						"type": "${json.preview_payment_methods[0].type}"
					},
					{
						"data": {
							"allow_saved_card": "${json.preview_payment_methods[1].data.allow_saved_card}",
							"available_cards": ["${json.preview_payment_methods[1].data.available_cards[0]}", "${json.preview_payment_methods[1].data.available_cards[1]}"],
							"do_save_card": "${json.preview_payment_methods[1].data.do_save_card}"
						},
						"id": "${json.preview_payment_methods[1].id}",
						"type": "${json.preview_payment_methods[1].type}"
					}
				],
				"required_fields": ["${json.required_fields[0]}", "${json.required_fields[1]}", "${json.required_fields[2]}", "${json.required_fields[3]}", "${json.required_fields[4]}", "${json.required_fields[5]}", "billing_address.care_of"],
				"shared": {
					"billing_address": {
						"care_of": "C/O Hakan Ostlund",
						"city": "AlingHelsingstadfors",
						"country": "${json.shared.billing_address.country}",
						"email": "${json.shared.billing_address.email}",
						"family_name": "Anglund",
						"given_name": "Eva InvoiceGreenNewSpec",
						"phone": "+46700012878",
						"postal_code": "${json.shared.billing_address.postal_code}",
						"street_address": "Sveavägen 44, 11111 Stockholm, Sweden Eriks Gata gatan"
					},
					"challenge": {
						"country": "${json.shared.challenge.country}",
						"email": "${json.shared.challenge.email}",
						"postal_code": "${json.shared.challenge.postal_code}"
					},
					"currency": "${json.shared.currency}",
					"customer": {
						"national_identification_number": "8910210312",
						"type": "${json.shared.customer.type}"
					},
					"language": "${json.shared.language}"
				},
				"status": {
					"prescreened": "${json.status.prescreened}",
					"require_terms_consent": "${json.status.require_terms_consent}"
				}
			}`,
			{
				"headers": {
					"Authorization": "Checkout otherStuffz",
					"User-Agent": "SysTest - perf",
					"Content-Type": "application/vnd.checkout.client-order-v1+json",
					"Accept": "application/vnd.checkout.server-order-v1+json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-other-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 200": (r) => r.status === 200 })) { return };
		json = JSON.parse(res.body);
		// Request #5
		res = http.post("https://some-other-host.example.com/yaco/orders/570714bf-3c2b-452e-90cd-f7c5e552bb25",
			`{
				"allowed_billing_countries": ["${json.allowed_billing_countries[0]}", "${json.allowed_billing_countries[1]}", "${json.allowed_billing_countries[2]}", "${json.allowed_billing_countries[3]}", "${json.allowed_billing_countries[4]}", "${json.allowed_billing_countries[5]}", "${json.allowed_billing_countries[6]}", "${json.allowed_billing_countries[7]}", "${json.allowed_billing_countries[8]}", "${json.allowed_billing_countries[9]}", "${json.allowed_billing_countries[10]}", "${json.allowed_billing_countries[11]}", "${json.allowed_billing_countries[12]}", "${json.allowed_billing_countries[13]}", "${json.allowed_billing_countries[14]}", "${json.allowed_billing_countries[15]}", "${json.allowed_billing_countries[16]}", "${json.allowed_billing_countries[17]}", "${json.allowed_billing_countries[18]}", "${json.allowed_billing_countries[19]}", "${json.allowed_billing_countries[20]}", "${json.allowed_billing_countries[21]}", "${json.allowed_billing_countries[22]}", "${json.allowed_billing_countries[23]}", "${json.allowed_billing_countries[24]}", "${json.allowed_billing_countries[25]}", "${json.allowed_billing_countries[26]}", "${json.allowed_billing_countries[27]}", "${json.allowed_billing_countries[28]}", "${json.allowed_billing_countries[29]}", "${json.allowed_billing_countries[30]}", "${json.allowed_billing_countries[31]}", "${json.allowed_billing_countries[32]}", "${json.allowed_billing_countries[33]}", "${json.allowed_billing_countries[34]}", "${json.allowed_billing_countries[35]}", "${json.allowed_billing_countries[36]}", "${json.allowed_billing_countries[37]}", "${json.allowed_billing_countries[38]}", "${json.allowed_billing_countries[39]}", "${json.allowed_billing_countries[40]}", "${json.allowed_billing_countries[41]}", "${json.allowed_billing_countries[42]}", "${json.allowed_billing_countries[43]}", "${json.allowed_billing_countries[44]}", "${json.allowed_billing_countries[45]}", "${json.allowed_billing_countries[46]}", "${json.allowed_billing_countries[47]}", "${json.allowed_billing_countries[48]}", "${json.allowed_billing_countries[49]}", "${json.allowed_billing_countries[50]}", "${json.allowed_billing_countries[51]}", "${json.allowed_billing_countries[52]}", "${json.allowed_billing_countries[53]}", "${json.allowed_billing_countries[54]}", "${json.allowed_billing_countries[55]}", "${json.allowed_billing_countries[56]}", "${json.allowed_billing_countries[57]}", "${json.allowed_billing_countries[58]}", "${json.allowed_billing_countries[59]}", "${json.allowed_billing_countries[60]}", "${json.allowed_billing_countries[61]}", "${json.allowed_billing_countries[62]}", "${json.allowed_billing_countries[63]}", "${json.allowed_billing_countries[64]}", "${json.allowed_billing_countries[65]}", "${json.allowed_billing_countries[66]}", "${json.allowed_billing_countries[67]}", "${json.allowed_billing_countries[68]}", "${json.allowed_billing_countries[69]}", "${json.allowed_billing_countries[70]}", "${json.allowed_billing_countries[71]}", "${json.allowed_billing_countries[72]}", "${json.allowed_billing_countries[73]}", "${json.allowed_billing_countries[74]}", "${json.allowed_billing_countries[75]}", "${json.allowed_billing_countries[76]}", "${json.allowed_billing_countries[77]}", "${json.allowed_billing_countries[78]}", "${json.allowed_billing_countries[79]}", "${json.allowed_billing_countries[80]}", "${json.allowed_billing_countries[81]}", "${json.allowed_billing_countries[82]}", "${json.allowed_billing_countries[83]}", "${json.allowed_billing_countries[84]}", "${json.allowed_billing_countries[85]}", "${json.allowed_billing_countries[86]}", "${json.allowed_billing_countries[87]}", "${json.allowed_billing_countries[88]}", "${json.allowed_billing_countries[89]}", "${json.allowed_billing_countries[90]}", "${json.allowed_billing_countries[91]}", "${json.allowed_billing_countries[92]}", "${json.allowed_billing_countries[93]}", "${json.allowed_billing_countries[94]}", "${json.allowed_billing_countries[95]}", "${json.allowed_billing_countries[96]}", "${json.allowed_billing_countries[97]}", "${json.allowed_billing_countries[98]}", "${json.allowed_billing_countries[99]}", "${json.allowed_billing_countries[100]}", "${json.allowed_billing_countries[101]}", "${json.allowed_billing_countries[102]}", "${json.allowed_billing_countries[103]}", "${json.allowed_billing_countries[104]}", "${json.allowed_billing_countries[105]}", "${json.allowed_billing_countries[106]}", "${json.allowed_billing_countries[107]}", "${json.allowed_billing_countries[108]}", "${json.allowed_billing_countries[109]}", "${json.allowed_billing_countries[110]}", "${json.allowed_billing_countries[111]}", "${json.allowed_billing_countries[112]}", "${json.allowed_billing_countries[113]}", "${json.allowed_billing_countries[114]}", "${json.allowed_billing_countries[115]}", "${json.allowed_billing_countries[116]}", "${json.allowed_billing_countries[117]}", "${json.allowed_billing_countries[118]}", "${json.allowed_billing_countries[119]}", "${json.allowed_billing_countries[120]}", "${json.allowed_billing_countries[121]}", "${json.allowed_billing_countries[122]}", "${json.allowed_billing_countries[123]}", "${json.allowed_billing_countries[124]}", "${json.allowed_billing_countries[125]}", "${json.allowed_billing_countries[126]}", "${json.allowed_billing_countries[127]}", "${json.allowed_billing_countries[128]}", "${json.allowed_billing_countries[129]}", "${json.allowed_billing_countries[130]}", "${json.allowed_billing_countries[131]}", "${json.allowed_billing_countries[132]}", "${json.allowed_billing_countries[133]}", "${json.allowed_billing_countries[134]}", "${json.allowed_billing_countries[135]}", "${json.allowed_billing_countries[136]}", "${json.allowed_billing_countries[137]}", "${json.allowed_billing_countries[138]}", "${json.allowed_billing_countries[139]}", "${json.allowed_billing_countries[140]}", "${json.allowed_billing_countries[141]}", "${json.allowed_billing_countries[142]}", "${json.allowed_billing_countries[143]}", "${json.allowed_billing_countries[144]}", "${json.allowed_billing_countries[145]}", "${json.allowed_billing_countries[146]}", "${json.allowed_billing_countries[147]}", "${json.allowed_billing_countries[148]}", "${json.allowed_billing_countries[149]}", "${json.allowed_billing_countries[150]}", "${json.allowed_billing_countries[151]}", "${json.allowed_billing_countries[152]}", "${json.allowed_billing_countries[153]}", "${json.allowed_billing_countries[154]}", "${json.allowed_billing_countries[155]}", "${json.allowed_billing_countries[156]}", "${json.allowed_billing_countries[157]}", "${json.allowed_billing_countries[158]}", "${json.allowed_billing_countries[159]}", "${json.allowed_billing_countries[160]}", "${json.allowed_billing_countries[161]}", "${json.allowed_billing_countries[162]}", "${json.allowed_billing_countries[163]}", "${json.allowed_billing_countries[164]}", "${json.allowed_billing_countries[165]}", "${json.allowed_billing_countries[166]}", "${json.allowed_billing_countries[167]}", "${json.allowed_billing_countries[168]}", "${json.allowed_billing_countries[169]}", "${json.allowed_billing_countries[170]}", "${json.allowed_billing_countries[171]}", "${json.allowed_billing_countries[172]}", "${json.allowed_billing_countries[173]}", "${json.allowed_billing_countries[174]}", "${json.allowed_billing_countries[175]}", "${json.allowed_billing_countries[176]}", "${json.allowed_billing_countries[177]}", "${json.allowed_billing_countries[178]}", "${json.allowed_billing_countries[179]}", "${json.allowed_billing_countries[180]}", "${json.allowed_billing_countries[181]}", "${json.allowed_billing_countries[182]}", "${json.allowed_billing_countries[183]}", "${json.allowed_billing_countries[184]}", "${json.allowed_billing_countries[185]}", "${json.allowed_billing_countries[186]}", "${json.allowed_billing_countries[187]}", "${json.allowed_billing_countries[188]}", "${json.allowed_billing_countries[189]}", "${json.allowed_billing_countries[190]}", "${json.allowed_billing_countries[191]}", "${json.allowed_billing_countries[192]}", "${json.allowed_billing_countries[193]}", "${json.allowed_billing_countries[194]}", "${json.allowed_billing_countries[195]}", "${json.allowed_billing_countries[196]}", "${json.allowed_billing_countries[197]}", "${json.allowed_billing_countries[198]}", "${json.allowed_billing_countries[199]}", "${json.allowed_billing_countries[200]}", "${json.allowed_billing_countries[201]}", "${json.allowed_billing_countries[202]}", "${json.allowed_billing_countries[203]}", "${json.allowed_billing_countries[204]}", "${json.allowed_billing_countries[205]}", "${json.allowed_billing_countries[206]}", "${json.allowed_billing_countries[207]}", "${json.allowed_billing_countries[208]}", "${json.allowed_billing_countries[209]}", "${json.allowed_billing_countries[210]}"],
				"allowed_shipping_countries": ["${json.allowed_shipping_countries[0]}", "${json.allowed_shipping_countries[1]}", "${json.allowed_shipping_countries[2]}", "${json.allowed_shipping_countries[3]}", "${json.allowed_shipping_countries[4]}", "${json.allowed_shipping_countries[5]}", "${json.allowed_shipping_countries[6]}", "${json.allowed_shipping_countries[7]}", "${json.allowed_shipping_countries[8]}", "${json.allowed_shipping_countries[9]}", "${json.allowed_shipping_countries[10]}", "${json.allowed_shipping_countries[11]}", "${json.allowed_shipping_countries[12]}", "${json.allowed_shipping_countries[13]}", "${json.allowed_shipping_countries[14]}", "${json.allowed_shipping_countries[15]}", "${json.allowed_shipping_countries[16]}", "${json.allowed_shipping_countries[17]}", "${json.allowed_shipping_countries[18]}", "${json.allowed_shipping_countries[19]}", "${json.allowed_shipping_countries[20]}", "${json.allowed_shipping_countries[21]}", "${json.allowed_shipping_countries[22]}", "${json.allowed_shipping_countries[23]}", "${json.allowed_shipping_countries[24]}", "${json.allowed_shipping_countries[25]}", "${json.allowed_shipping_countries[26]}", "${json.allowed_shipping_countries[27]}", "${json.allowed_shipping_countries[28]}", "${json.allowed_shipping_countries[29]}", "${json.allowed_shipping_countries[30]}", "${json.allowed_shipping_countries[31]}", "${json.allowed_shipping_countries[32]}", "${json.allowed_shipping_countries[33]}", "${json.allowed_shipping_countries[34]}", "${json.allowed_shipping_countries[35]}", "${json.allowed_shipping_countries[36]}", "${json.allowed_shipping_countries[37]}", "${json.allowed_shipping_countries[38]}", "${json.allowed_shipping_countries[39]}", "${json.allowed_shipping_countries[40]}", "${json.allowed_shipping_countries[41]}", "${json.allowed_shipping_countries[42]}", "${json.allowed_shipping_countries[43]}", "${json.allowed_shipping_countries[44]}", "${json.allowed_shipping_countries[45]}", "${json.allowed_shipping_countries[46]}", "${json.allowed_shipping_countries[47]}", "${json.allowed_shipping_countries[48]}", "${json.allowed_shipping_countries[49]}", "${json.allowed_shipping_countries[50]}", "${json.allowed_shipping_countries[51]}", "${json.allowed_shipping_countries[52]}", "${json.allowed_shipping_countries[53]}", "${json.allowed_shipping_countries[54]}", "${json.allowed_shipping_countries[55]}", "${json.allowed_shipping_countries[56]}", "${json.allowed_shipping_countries[57]}", "${json.allowed_shipping_countries[58]}", "${json.allowed_shipping_countries[59]}", "${json.allowed_shipping_countries[60]}", "${json.allowed_shipping_countries[61]}", "${json.allowed_shipping_countries[62]}", "${json.allowed_shipping_countries[63]}", "${json.allowed_shipping_countries[64]}", "${json.allowed_shipping_countries[65]}", "${json.allowed_shipping_countries[66]}", "${json.allowed_shipping_countries[67]}", "${json.allowed_shipping_countries[68]}", "${json.allowed_shipping_countries[69]}", "${json.allowed_shipping_countries[70]}", "${json.allowed_shipping_countries[71]}", "${json.allowed_shipping_countries[72]}", "${json.allowed_shipping_countries[73]}", "${json.allowed_shipping_countries[74]}", "${json.allowed_shipping_countries[75]}", "${json.allowed_shipping_countries[76]}", "${json.allowed_shipping_countries[77]}", "${json.allowed_shipping_countries[78]}", "${json.allowed_shipping_countries[79]}", "${json.allowed_shipping_countries[80]}", "${json.allowed_shipping_countries[81]}", "${json.allowed_shipping_countries[82]}", "${json.allowed_shipping_countries[83]}", "${json.allowed_shipping_countries[84]}", "${json.allowed_shipping_countries[85]}", "${json.allowed_shipping_countries[86]}", "${json.allowed_shipping_countries[87]}", "${json.allowed_shipping_countries[88]}", "${json.allowed_shipping_countries[89]}", "${json.allowed_shipping_countries[90]}", "${json.allowed_shipping_countries[91]}", "${json.allowed_shipping_countries[92]}", "${json.allowed_shipping_countries[93]}", "${json.allowed_shipping_countries[94]}", "${json.allowed_shipping_countries[95]}", "${json.allowed_shipping_countries[96]}", "${json.allowed_shipping_countries[97]}", "${json.allowed_shipping_countries[98]}", "${json.allowed_shipping_countries[99]}", "${json.allowed_shipping_countries[100]}", "${json.allowed_shipping_countries[101]}", "${json.allowed_shipping_countries[102]}", "${json.allowed_shipping_countries[103]}", "${json.allowed_shipping_countries[104]}", "${json.allowed_shipping_countries[105]}", "${json.allowed_shipping_countries[106]}", "${json.allowed_shipping_countries[107]}", "${json.allowed_shipping_countries[108]}", "${json.allowed_shipping_countries[109]}", "${json.allowed_shipping_countries[110]}", "${json.allowed_shipping_countries[111]}", "${json.allowed_shipping_countries[112]}", "${json.allowed_shipping_countries[113]}", "${json.allowed_shipping_countries[114]}", "${json.allowed_shipping_countries[115]}", "${json.allowed_shipping_countries[116]}", "${json.allowed_shipping_countries[117]}", "${json.allowed_shipping_countries[118]}", "${json.allowed_shipping_countries[119]}", "${json.allowed_shipping_countries[120]}", "${json.allowed_shipping_countries[121]}", "${json.allowed_shipping_countries[122]}", "${json.allowed_shipping_countries[123]}", "${json.allowed_shipping_countries[124]}", "${json.allowed_shipping_countries[125]}", "${json.allowed_shipping_countries[126]}", "${json.allowed_shipping_countries[127]}", "${json.allowed_shipping_countries[128]}", "${json.allowed_shipping_countries[129]}", "${json.allowed_shipping_countries[130]}", "${json.allowed_shipping_countries[131]}", "${json.allowed_shipping_countries[132]}", "${json.allowed_shipping_countries[133]}", "${json.allowed_shipping_countries[134]}", "${json.allowed_shipping_countries[135]}", "${json.allowed_shipping_countries[136]}", "${json.allowed_shipping_countries[137]}", "${json.allowed_shipping_countries[138]}", "${json.allowed_shipping_countries[139]}", "${json.allowed_shipping_countries[140]}", "${json.allowed_shipping_countries[141]}", "${json.allowed_shipping_countries[142]}", "${json.allowed_shipping_countries[143]}", "${json.allowed_shipping_countries[144]}", "${json.allowed_shipping_countries[145]}", "${json.allowed_shipping_countries[146]}", "${json.allowed_shipping_countries[147]}", "${json.allowed_shipping_countries[148]}", "${json.allowed_shipping_countries[149]}", "${json.allowed_shipping_countries[150]}", "${json.allowed_shipping_countries[151]}", "${json.allowed_shipping_countries[152]}", "${json.allowed_shipping_countries[153]}", "${json.allowed_shipping_countries[154]}", "${json.allowed_shipping_countries[155]}", "${json.allowed_shipping_countries[156]}", "${json.allowed_shipping_countries[157]}", "${json.allowed_shipping_countries[158]}", "${json.allowed_shipping_countries[159]}", "${json.allowed_shipping_countries[160]}", "${json.allowed_shipping_countries[161]}", "${json.allowed_shipping_countries[162]}", "${json.allowed_shipping_countries[163]}", "${json.allowed_shipping_countries[164]}", "${json.allowed_shipping_countries[165]}", "${json.allowed_shipping_countries[166]}", "${json.allowed_shipping_countries[167]}", "${json.allowed_shipping_countries[168]}", "${json.allowed_shipping_countries[169]}", "${json.allowed_shipping_countries[170]}", "${json.allowed_shipping_countries[171]}", "${json.allowed_shipping_countries[172]}", "${json.allowed_shipping_countries[173]}", "${json.allowed_shipping_countries[174]}", "${json.allowed_shipping_countries[175]}", "${json.allowed_shipping_countries[176]}", "${json.allowed_shipping_countries[177]}", "${json.allowed_shipping_countries[178]}", "${json.allowed_shipping_countries[179]}", "${json.allowed_shipping_countries[180]}", "${json.allowed_shipping_countries[181]}", "${json.allowed_shipping_countries[182]}", "${json.allowed_shipping_countries[183]}", "${json.allowed_shipping_countries[184]}", "${json.allowed_shipping_countries[185]}", "${json.allowed_shipping_countries[186]}", "${json.allowed_shipping_countries[187]}", "${json.allowed_shipping_countries[188]}", "${json.allowed_shipping_countries[189]}", "${json.allowed_shipping_countries[190]}", "${json.allowed_shipping_countries[191]}", "${json.allowed_shipping_countries[192]}", "${json.allowed_shipping_countries[193]}", "${json.allowed_shipping_countries[194]}", "${json.allowed_shipping_countries[195]}", "${json.allowed_shipping_countries[196]}", "${json.allowed_shipping_countries[197]}", "${json.allowed_shipping_countries[198]}", "${json.allowed_shipping_countries[199]}", "${json.allowed_shipping_countries[200]}", "${json.allowed_shipping_countries[201]}", "${json.allowed_shipping_countries[202]}", "${json.allowed_shipping_countries[203]}", "${json.allowed_shipping_countries[204]}", "${json.allowed_shipping_countries[205]}", "${json.allowed_shipping_countries[206]}", "${json.allowed_shipping_countries[207]}", "${json.allowed_shipping_countries[208]}", "${json.allowed_shipping_countries[209]}", "${json.allowed_shipping_countries[210]}"],
				"analytics_user_id": "${json.analytics_user_id}",
				"available_payment_methods": [
					{
						"data": {
							"days": "${json.available_payment_methods[0].data.days}"
						},
						"id": "${json.available_payment_methods[0].id}",
						"type": "${json.available_payment_methods[0].type}"
					}
				],
				"cart": {
					"items": [
						{
							"image_url": "${json.cart.items[0].image_url}",
							"name": "${json.cart.items[0].name}",
							"product_url": "${json.cart.items[0].product_url}",
							"quantity": "${json.cart.items[0].quantity}",
							"reference": "${json.cart.items[0].reference}",
							"tax_rate": "${json.cart.items[0].tax_rate}",
							"total_price_excluding_tax": "${json.cart.items[0].total_price_excluding_tax}",
							"total_price_including_tax": "${json.cart.items[0].total_price_including_tax}",
							"total_tax_amount": "${json.cart.items[0].total_tax_amount}",
							"type": "${json.cart.items[0].type}",
							"unit_price": "${json.cart.items[0].unit_price}"
						}
					],
					"subtotal": "${json.cart.subtotal}",
					"total_discount_amount_excluding_tax": "${json.cart.total_discount_amount_excluding_tax}",
					"total_price_excluding_tax": "${json.cart.total_price_excluding_tax}",
					"total_price_including_tax": "${json.cart.total_price_including_tax}",
					"total_shipping_amount_excluding_tax": "${json.cart.total_shipping_amount_excluding_tax}",
					"total_store_credit": "${json.cart.total_store_credit}",
					"total_surcharge_amount_excluding_tax": "${json.cart.total_surcharge_amount_excluding_tax}",
					"total_tax_amount": "${json.cart.total_tax_amount}"
				},
				"correlation_id": "a6c51342-b107-4463-a2a0-b530f1bac03e",
				"merchant_urls": {
					"checkout": "${json.merchant_urls.checkout}",
					"confirmation": "${json.merchant_urls.confirmation}",
					"terms": "${json.merchant_urls.terms}"
				},
				"options": {
					"allow_separate_shipping_address": "${json.options.allow_separate_shipping_address}",
					"allowed_customer_types": ["${json.options.allowed_customer_types[0]}"],
					"date_of_birth_mandatory": "${json.options.date_of_birth_mandatory}",
					"national_identification_number_mandatory": "${json.options.national_identification_number_mandatory}",
					"payment_selector_on_load": "${json.options.payment_selector_on_load}"
				},
				"shared": {
					"billing_address": {
						"care_of": "${json.shared.billing_address.care_of}",
						"city": "${json.shared.billing_address.city}",
						"country": "${json.shared.billing_address.country}",
						"email": "${json.shared.billing_address.email}",
						"family_name": "${json.shared.billing_address.family_name}",
						"given_name": "${json.shared.billing_address.given_name}",
						"phone": "${json.shared.billing_address.phone}",
						"postal_code": "${json.shared.billing_address.postal_code}",
						"street_address": "${json.shared.billing_address.street_address}",
						"street_address2": "${json.shared.billing_address.street_address2}"
					},
					"challenge": {
						"country": "${json.shared.challenge.country}",
						"email": "${json.shared.challenge.email}",
						"postal_code": "${json.shared.challenge.postal_code}"
					},
					"currency": "${json.shared.currency}",
					"customer": {
						"national_identification_number": "${json.shared.customer.national_identification_number}",
						"type": "${json.shared.customer.type}"
					},
					"language": "${json.shared.language}",
					"selected_payment_method": {
						"data": {
							"days": 14
						},
						"id": "-1",
						"type": "invoice"
					}
				},
				"status": {
					"prescreened": "${json.status.prescreened}",
					"require_terms_consent": "${json.status.require_terms_consent}"
				}
			}`,
			{
				"headers": {
					"Authorization": "Checkout otherStuffz",
					"User-Agent": "SysTest - perf",
					"Content-Type": "application/vnd.checkout.client-order-v1+json",
					"Accept": "application/vnd.checkout.server-order-v1+json",
					"Accept-Encoding": "gzip;q=1.0,deflate;q=0.6,identity;q=0.3",
					"Host": "some-other-host.example.com"
				}
			}
		)
		if (!check(res, {"status is 200": (r) => r.status === 200 })) { return };
		json = JSON.parse(res.body);
		// Request #6
		res = http.connect("https://a-third-host.example.com:3000",
		""
		)
	});

}
