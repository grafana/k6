/*
 *
 * This script is a user journey for http://loadimpactair.guldskeden.se
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

import { group, sleep } from "k6";
import { parseHTML } from "k6/html";
import http from "k6/http";

/**
 * Returns random number between (inclusive) min and max
 */
function rnd(min, max) {
  min = Math.ceil(min);
  max = Math.floor(max);

  return Math.floor(Math.random() * (max - min + 1)) + min;
}

/**
 * Returns __VIEWSTATE found in body or empty string if not found
 */
function findViewstate(body) {
	var strViewstate = "";
	var bExists = false;

	[strViewstate, bExists] = parseHTML(body).find("#__VIEWSTATE").attr("value");

	if (!bExists)
	{
		console.log("No viewstate");
		strViewstate = "";
	}

	return strViewstate;
}

/**
 * Returns __EVENTVALIDATION found in body or empty string if not found
 */
function findEventvalidation(body) {
	var strEVentvalidation = "";
	var bExists = false;

	[strEventvalidation, bExists] = parseHTML(body).find("#__EVENTVALIDATION").attr("value");

	if (!bExists)
	{
		console.log("No eventvalidation");
		strEventvalidation = "";
	}

	return strEventvalidation;
}


/**
 * Returns __VIEWSTATE found in body of Ajax call return (i.e. panel) or empty string if not found
 */
function findPanelViewstate(body) {
	var strViewstate = "";
	var findVS = new RegExp("\\|__VIEWSTATE\\|(.*?)\\|");
	var reMatch = findVS.exec(body);
	if (reMatch)
	{
		strViewstate = reMatch[1];
	}
	else
	{
		console.log("No panel viewstate");
		console.log(body);
	}

	return strViewstate;
}


/**
 * Returns __EVENTVALIDATION found in body of Ajax call return (i.e. panel) or empty string if not found
 */
function findPanelEventvalidation(body) {
	var strEventvalidation = "";
	var findEV = new RegExp("\\|__EVENTVALIDATION\\|(.*?)\\|");
	var reMatch = findEV.exec(body);
	if (reMatch)
	{
		strEventvalidation = reMatch[1];
	}
	else
	{
		console.log("No panel eventvalidation");
		console.log(body);
	}

	return strEventvalidation;
}



/**
 * Set k6 test execution options
 * vusMax: Maximum number of vus that can be executed in a test
 * duration: the duration of the test run
 * maxRedirects: the number redirects allowed for a request before it is considered failed
 * insecureSkipTLSVerify: true will allow all certificates, dev and homemade as well
 * stages: from 0 to 10 vu in 20 seconds
 * 		   from 10 to 15 vu in 20 seconds
 * 		   from 15 to 20 vu in 20 seconds
 * 		   stay at 20 vu for 40 seconds
 * 		   from 20 to 80 vu in 20 seconds
 * 		   stay at 80 vu for 180 seconds
 * 	  	   from 80 to 0 vu in 10 seconds
 */
export let options =  {
	vusMax: 100,
	duration: "10m",
	stages: 
		[{target: 10, duration: "20s"},
		 {target: 15, duration: "20s"},
		 {target: 20, duration: "20s"},
		 {target: 20, duration: "40s"},
		 {target: 80, duration: "20s"},
		 {target: 80, duration: "180s"},
		 {target: 0, duration: "10s"}],	
	maxRedirects: 15,
	insecureSkipTLSVerify: true,
}



// Use a top-level function wrapper to allow us to return from it. If you don't need to
// return from your script at any point, you can skip the wrapper - VU scopes are isolated.
export default function() {

	// Something to hold the responses in
	var res = null;

	// First step - go to the landing page
	// Put in group and name group something resaonable
	// Batches are from how the real page is actually loaded
	//   first the page then the next level content
	//   and the last batch is the content from the bottom level of the content hierarchy
	group ("01_landingpage", function () {
		http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/"}
		]);
		http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/dtagent630_q_1268.js"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/css/Style.css"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/bundles/modernizr?v=wBEWDufH_8Md-Pbioxomt90vm6tJN2Pyy9u9zHtWsPo1"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Content/css?v=PUDFxlRUUS8e8pp6Y9WeVnF_4RmJM7BwtYyTz0D-zu81"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Media/Images/milehightheader.jpg"},
		]);
		http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Media/Images/pattern.png"}
		]);
	});

	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5, 10));

	// Second step - go to booking page
	// Put in group and name group something resaonable
	// Keep response of reqest in res variable
	group ("02_toBooking", function () {
		res = http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Pages/Booking.aspx"}
		]);

		http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/WebResource.axd?d=pynGkmcFUV13He1Qd6_TZIYhwal-_oKtgDTk-WGCThe4WaERcXrvazAgcpzILVEKBqxXIOn8SqRDbHeXAGVc5w2&t=636161530540000000"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Scripts/WebForms/MsAjax/MicrosoftAjax.js"},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Scripts/WebForms/MsAjax/MicrosoftAjaxWebForms.js"},
		]);
	});

	// Get __VIEWSTATE and __EVENTVALIDATION from the response body
	var strViewstate = findViewstate(res[0].body);
    var strEventvalidation = findEventvalidation(res[0].body);

	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5,10));

	// Third step - select departure location
	// Put in group and name group something resaonable
	// Use __VIEWSTATE and __EVENTVALIDATION from previous response in this request
	// Keep response of reqest in res variable
	group ("03_departure", function () {
		res = http.batch([
			{"method" : "POST", 
			 "url" : "http://loadimpactair.guldskeden.se/Pages/Booking", "body" : {"__EVENTTARGET":"ctl00$MainContent$ddlDeparture","__EVENTARGUMENT":"","__LASTFOCUS":"","__VIEWSTATE":strViewstate, "__VIEWSTATEGENERATOR":"A4196A29","__EVENTVALIDATION":strEventvalidation,"ctl00$MainContent$ddlDeparture":"1","ctl00$MainContent$ddlDestination":"- Select -"},
			 "params" : { headers: { "Content-Type" : "application/x-www-form-urlencoded"} }}
		]);
	});

	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5, 10));

	// Get next __VIEWSTATE and __EVENTVALIDATION from the response body
	strViewstate = findViewstate(res[0].body);
    strEventvalidation = findEventvalidation(res[0].body);

	// Fourth step - select destination location
	// Put in group and name group something resaonable
	// Use __VIEWSTATE and __EVENTVALIDATION from previous response in this request
	// Keep response of reqest in res variable
	group ("04_destination", function () {
		res = http.batch([
			{"method" : "POST", 
			 "url" : "http://loadimpactair.guldskeden.se/Pages/Booking", 
			 "body" :  {"__EVENTTARGET":"ctl00$MainContent$ddlDestination","__EVENTARGUMENT":"","__LASTFOCUS":"","__VIEWSTATE" : strViewstate,"__VIEWSTATEGENERATOR":"A4196A29","__EVENTVALIDATION": strEventvalidation,"ctl00$MainContent$ddlDeparture":1,"ctl00$MainContent$ddlDestination":3},
			 "params" : { headers: { "Content-Type" : "application/x-www-form-urlencoded"} }},
		]);
	});


	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5, 10));

	// Get next __VIEWSTATE and __EVENTVALIDATION from the response body
	strViewstate = findViewstate(res[0].body);
    strEventvalidation = findEventvalidation(res[0].body);

	// Fifth step - select departure date
	// Put in group and name group something resaonable
	// Use __VIEWSTATE and __EVENTVALIDATION from previous response in this request
	// Use the header to set the user agent as well because some Ajax implementations are very picky on the user agent
	// Keep response of reqest in res variable
	group ("05_departdate", function () {
		res = http.batch([
			{"method" : "POST", "url" : "http://loadimpactair.guldskeden.se/Pages/Booking", 
			 "body" : {"ctl00$MainContent$ctl00":"ctl00$MainContent$ctl02|ctl00$MainContent$DateDeparture","__EVENTTARGET":"ctl00$MainContent$DateDeparture","__EVENTARGUMENT":"6294","__LASTFOCUS":"","__VIEWSTATE":strViewstate,"__VIEWSTATEGENERATOR":"A4196A29","__EVENTVALIDATION":strEventvalidation,"ctl00$MainContent$ddlDeparture":"1","ctl00$MainContent$ddlDestination":"3","__ASYNCPOST":"true","":""},
			 "params" : { headers: { "Content-Type" : "application/x-www-form-urlencoded; charset=UTF-8",
									"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.79 Safari/537.36 Edge/14.14393" } }}
		]);
	});

	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5, 10));

	// Get next __VIEWSTATE and __EVENTVALIDATION from the AJAX call response body
	strViewstate = findPanelViewstate(res[0].body);
    strEventvalidation = findPanelEventvalidation(res[0].body);

	// Sixth step - select return date
	// Put in group and name group something resaonable
	// Use __VIEWSTATE and __EVENTVALIDATION from previous response in this request
	// Use the header to set the user agent as well because some Ajax implementations are very picky on the user agent
	// Keep response of reqest in res variable
	group ("06_returndate", function () {
		res = http.batch([
			{"method" : "POST", 
			 "url" : "http://loadimpactair.guldskeden.se/Pages/Booking", 
			 "body" :  {"ctl00$MainContent$ctl00":"ctl00$MainContent$ctl04|ctl00$MainContent$DateReturn","ctl00$MainContent$ddlDeparture":"1","ctl00$MainContent$ddlDestination":"3","__EVENTTARGET":"ctl00$MainContent$DateReturn","__EVENTARGUMENT":"6298","__LASTFOCUS":"","__VIEWSTATE":strViewstate,"__VIEWSTATEGENERATOR":"A4196A29","__EVENTVALIDATION":strEventvalidation,"__ASYNCPOST":"true","":""},
			 "params" : { headers: { "Content-Type" : "application/x-www-form-urlencoded; charset=UTF-8", 
									 "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.79 Safari/537.36 Edge/14.14393" } }}
		]);
	});

	// Sleep between 5-10 seconds (the amount of time an average user would read before the next step/action)
	sleep(rnd(5, 10));

	// Get next __VIEWSTATE and __EVENTVALIDATION from the AJAX call response body
	strViewstate = findPanelViewstate(res[0].body);
    strEventvalidation = findPanelEventvalidation(res[0].body);

	// Seventh step - search for available flights
	// Put in group and name group something resaonable
	// Use __VIEWSTATE and __EVENTVALIDATION from previous response in this request
	group ("07_doSearch", function () {
		http.batch([
			{"method" : "POST", 
			 "url" : "http://loadimpactair.guldskeden.se/Pages/Booking", 
			 "body" : {"ctl00$MainContent$ctl00":"ctl00$MainContent$ctl06|ctl00$MainContent$ctl08","ctl00$MainContent$ddlDeparture":"1","ctl00$MainContent$ddlDestination":"3","__EVENTTARGET":"","__EVENTARGUMENT":"","__LASTFOCUS":"","__VIEWSTATE":strViewstate,"__VIEWSTATEGENERATOR":"A4196A29","__EVENTVALIDATION":strEventvalidation,"__ASYNCPOST":"true","ctl00$MainContent$ctl08":"Search flights"},
			 "params" : { headers: { "Content-Type" : "application/x-www-form-urlencoded; charset=UTF-8"} }},
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/Pages/Flights.aspx"}
		]);

		http.batch([
			{"method" : "GET", "url" : "http://loadimpactair.guldskeden.se/css/Pace.css"},
		]);
	});

	// Next iteration will start as soon as this iterations end
	// Final sleep controls the pacing of the iterations
	// Empirical evidence from more than 2 M tests executed show that 20-40 seconds is a reasonable default
    sleep(rnd(20, 40));
    return;
}

