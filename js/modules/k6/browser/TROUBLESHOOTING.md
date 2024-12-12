# Troubleshooting

If you're having issues installing or running xk6-browser, this document is a good place to start. If your issue is not mentioned here, please see [SUPPORT.md](/SUPPORT.md).

## Timeout error launching the browser

If you're using Ubuntu, including under Microsoft's WSL2, and getting the following error consistently (i.e. in every test run):

> `launching browser: getting DevTools URL: timed out after 30s`

Confirm that you don't have the `chromium-browser` package installed. This should return no results:

```shell
dpkg -l | grep '^ii  chromium-browser'
```

On recent versions of Ubuntu (>=19.10), this is a transitional DEB package for the Snap Chromium package.

Running the browser in a container like Snap or Flatpak is not supported by xk6-browser.

To resolve this, remove the `chromium-browser` package, and install a native DEB package. Since Ubuntu doesn't carry one in their repositories, you will need to add an external repository that does. We recommend only using trusted repositories, preferably from Google itself.

If you're OK with using Google Chrome instead of Chromium, run the following commands:

```shell
sudo apt remove -y chromium-browser
wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
sudo apt update && sudo apt install -y google-chrome-stable
```

Then try running the xk6-browser test again.
