package common

// Device represents an end-user device (computer, tablet, phone etc.)
type Device struct {
	Name              string   `js:"name"`
	UserAgent         string   `js:"userAgent"`
	Viewport          Viewport `js:"viewport"`
	DeviceScaleFactor float64  `js:"deviceScaleFactor"`
	IsMobile          bool     `js:"isMobile"`
	HasTouch          bool     `js:"hasTouch"`
}

// GetDevices returns predefined emulation settings for many end-user devices.
//
//nolint:lll,funlen
func GetDevices() map[string]Device {
	return map[string]Device{
		"Blackberry PlayBook": {
			Name:      "Blackberry PlayBook",
			UserAgent: "Mozilla/5.0 (PlayBook; U; RIM Tablet OS 2.1.0; en-US) AppleWebKit/536.2+ (KHTML like Gecko) Version/7.2.1.0 Safari/536.2+",
			Viewport: Viewport{
				Width:  600,
				Height: 1024,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Blackberry PlayBook landscape": {
			Name:      "Blackberry PlayBook landscape",
			UserAgent: "Mozilla/5.0 (PlayBook; U; RIM Tablet OS 2.1.0; en-US) AppleWebKit/536.2+ (KHTML like Gecko) Version/7.2.1.0 Safari/536.2+",
			Viewport: Viewport{
				Width:  1024,
				Height: 600,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"BlackBerry Z30": {
			Name:      "BlackBerry Z30",
			UserAgent: "Mozilla/5.0 (BB10; Touch) AppleWebKit/537.10+ (KHTML, like Gecko) Version/10.0.9.2372 Mobile Safari/537.10+",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"BlackBerry Z30 landscape": {
			Name:      "BlackBerry Z30 landscape",
			UserAgent: "Mozilla/5.0 (BB10; Touch) AppleWebKit/537.10+ (KHTML, like Gecko) Version/10.0.9.2372 Mobile Safari/537.10+",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy Note 3": {
			Name:      "Galaxy Note 3",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.3; en-us; SM-N900T Build/JSS15J) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy Note 3 landscape": {
			Name:      "Galaxy Note 3 landscape",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.3; en-us; SM-N900T Build/JSS15J) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy Note II": {
			Name:      "Galaxy Note II",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.1; en-us; GT-N7100 Build/JRO03C) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy Note II landscape": {
			Name:      "Galaxy Note II landscape",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.1; en-us; GT-N7100 Build/JRO03C) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy S III": {
			Name:      "Galaxy S III",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.0; en-us; GT-I9300 Build/IMM76D) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy S III landscape": {
			Name:      "Galaxy S III landscape",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.0; en-us; GT-I9300 Build/IMM76D) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy S5": {
			Name:      "Galaxy S5",
			UserAgent: "Mozilla/5.0 (Linux; Android 5.0; SM-G900P Build/LRX21T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Galaxy S5 landscape": {
			Name:      "Galaxy S5 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 5.0; SM-G900P Build/LRX21T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad": {
			Name:      "iPad",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  768,
				Height: 1024,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad landscape": {
			Name:      "iPad landscape",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  1024,
				Height: 768,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad Mini": {
			Name:      "iPad Mini",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  768,
				Height: 1024,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad Mini landscape": {
			Name:      "iPad Mini landscape",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  1024,
				Height: 768,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad Pro": {
			Name:      "iPad Pro",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  1024,
				Height: 1366,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPad Pro landscape": {
			Name:      "iPad Pro landscape",
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 11_0 like Mac OS X) AppleWebKit/604.1.34 (KHTML, like Gecko) Version/11.0 Mobile/15A5341f Safari/604.1",
			Viewport: Viewport{
				Width:  1366,
				Height: 1024,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 4": {
			Name:      "iPhone 4",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1_2 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D257 Safari/9537.53",
			Viewport: Viewport{
				Width:  320,
				Height: 480,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 4 landscape": {
			Name:      "iPhone 4 landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1_2 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D257 Safari/9537.53",
			Viewport: Viewport{
				Width:  480,
				Height: 320,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 5": {
			Name:      "iPhone 5",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3_1 like Mac OS X) AppleWebKit/603.1.30 (KHTML, like Gecko) Version/10.0 Mobile/14E304 Safari/602.1",
			Viewport: Viewport{
				Width:  320,
				Height: 568,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 5 landscape": {
			Name:      "iPhone 5 landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3_1 like Mac OS X) AppleWebKit/603.1.30 (KHTML, like Gecko) Version/10.0 Mobile/14E304 Safari/602.1",
			Viewport: Viewport{
				Width:  568,
				Height: 320,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 6": {
			Name:      "iPhone 6",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  375,
				Height: 667,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 6 landscape": {
			Name:      "iPhone 6 landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  667,
				Height: 375,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 6 Plus": {
			Name:      "iPhone 6 Plus",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  414,
				Height: 736,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 6 Plus landscape": {
			Name:      "iPhone 6 Plus landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  736,
				Height: 414,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 7": {
			Name:      "iPhone 7",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  375,
				Height: 667,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 7 landscape": {
			Name:      "iPhone 7 landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  667,
				Height: 375,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 7 Plus": {
			Name:      "iPhone 7 Plus",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  414,
				Height: 736,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 7 Plus landscape": {
			Name:      "iPhone 7 Plus landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  736,
				Height: 414,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 8": {
			Name:      "iPhone 8",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  375,
				Height: 667,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 8 landscape": {
			Name:      "iPhone 8 landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  667,
				Height: 375,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 8 Plus": {
			Name:      "iPhone 8 Plus",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  414,
				Height: 736,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone 8 Plus landscape": {
			Name:      "iPhone 8 Plus landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  736,
				Height: 414,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone SE": {
			Name:      "iPhone SE",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3_1 like Mac OS X) AppleWebKit/603.1.30 (KHTML, like Gecko) Version/10.0 Mobile/14E304 Safari/602.1",
			Viewport: Viewport{
				Width:  320,
				Height: 568,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone SE landscape": {
			Name:      "iPhone SE landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3_1 like Mac OS X) AppleWebKit/603.1.30 (KHTML, like Gecko) Version/10.0 Mobile/14E304 Safari/602.1",
			Viewport: Viewport{
				Width:  568,
				Height: 320,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone X": {
			Name:      "iPhone X",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  375,
				Height: 812,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone X landscape": {
			Name:      "iPhone X landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
			Viewport: Viewport{
				Width:  812,
				Height: 375,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone XR": {
			Name:      "iPhone XR",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 12_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.0 Mobile/15E148 Safari/604.1",
			Viewport: Viewport{
				Width:  414,
				Height: 896,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"iPhone XR landscape": {
			Name:      "iPhone XR landscape",
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 12_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.0 Mobile/15E148 Safari/604.1",
			Viewport: Viewport{
				Width:  896,
				Height: 414,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"JioPhone 2": {
			Name:      "JioPhone 2",
			UserAgent: "Mozilla/5.0 (Mobile; LYF/F300B/LYF-F300B-001-01-15-130718-i;Android; rv:48.0) Gecko/48.0 Firefox/48.0 KAIOS/2.5",
			Viewport: Viewport{
				Width:  240,
				Height: 320,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"JioPhone 2 landscape": {
			Name:      "JioPhone 2 landscape",
			UserAgent: "Mozilla/5.0 (Mobile; LYF/F300B/LYF-F300B-001-01-15-130718-i;Android; rv:48.0) Gecko/48.0 Firefox/48.0 KAIOS/2.5",
			Viewport: Viewport{
				Width:  320,
				Height: 240,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Kindle Fire HDX": {
			Name:      "Kindle Fire HDX",
			UserAgent: "Mozilla/5.0 (Linux; U; en-us; KFAPWI Build/JDQ39) AppleWebKit/535.19 (KHTML, like Gecko) Silk/3.13 Safari/535.19 Silk-Accelerated=true",
			Viewport: Viewport{
				Width:  800,
				Height: 1280,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Kindle Fire HDX landscape": {
			Name:      "Kindle Fire HDX landscape",
			UserAgent: "Mozilla/5.0 (Linux; U; en-us; KFAPWI Build/JDQ39) AppleWebKit/535.19 (KHTML, like Gecko) Silk/3.13 Safari/535.19 Silk-Accelerated=true",
			Viewport: Viewport{
				Width:  1280,
				Height: 800,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"LG Optimus L70": {
			Name:      "LG Optimus L70",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.4.2; en-us; LGMS323 Build/KOT49I.MS32310c) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  384,
				Height: 640,
			},
			DeviceScaleFactor: 1.25,
			IsMobile:          true,
			HasTouch:          true,
		},
		"LG Optimus L70 landscape": {
			Name:      "LG Optimus L70 landscape",
			UserAgent: "Mozilla/5.0 (Linux; U; Android 4.4.2; en-us; LGMS323 Build/KOT49I.MS32310c) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  640,
				Height: 384,
			},
			DeviceScaleFactor: 1.25,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Microsoft Lumia 550": {
			Name:      "Microsoft Lumia 550",
			UserAgent: "Mozilla/5.0 (Windows Phone 10.0; Android 4.2.1; Microsoft; Lumia 550) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/46.0.2486.0 Mobile Safari/537.36 Edge/14.14263",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Microsoft Lumia 950": {
			Name:      "Microsoft Lumia 950",
			UserAgent: "Mozilla/5.0 (Windows Phone 10.0; Android 4.2.1; Microsoft; Lumia 950) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/46.0.2486.0 Mobile Safari/537.36 Edge/14.14263",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 4,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Microsoft Lumia 950 landscape": {
			Name:      "Microsoft Lumia 950 landscape",
			UserAgent: "Mozilla/5.0 (Windows Phone 10.0; Android 4.2.1; Microsoft; Lumia 950) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/46.0.2486.0 Mobile Safari/537.36 Edge/14.14263",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 4,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 10": {
			Name:      "Nexus 10",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 10 Build/MOB31T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Safari/537.36",
			Viewport: Viewport{
				Width:  800,
				Height: 1280,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 10 landscape": {
			Name:      "Nexus 10 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 10 Build/MOB31T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Safari/537.36",
			Viewport: Viewport{
				Width:  1280,
				Height: 800,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 4": {
			Name:      "Nexus 4",
			UserAgent: "Mozilla/5.0 (Linux; Android 4.4.2; Nexus 4 Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  384,
				Height: 640,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 4 landscape": {
			Name:      "Nexus 4 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 4.4.2; Nexus 4 Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  640,
				Height: 384,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 5": {
			Name:      "Nexus 5",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  360,
				Height: 640,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 5 landscape": {
			Name:      "Nexus 5 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  640,
				Height: 360,
			},
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 5X": {
			Name:      "Nexus 5X",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Nexus 5X Build/OPR4.170623.006) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  412,
				Height: 732,
			},
			DeviceScaleFactor: 2.625,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 5X landscape": {
			Name:      "Nexus 5X landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Nexus 5X Build/OPR4.170623.006) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  732,
				Height: 412,
			},
			DeviceScaleFactor: 2.625,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 6": {
			Name:      "Nexus 6",
			UserAgent: "Mozilla/5.0 (Linux; Android 7.1.1; Nexus 6 Build/N6F26U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  412,
				Height: 732,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 6 landscape": {
			Name:      "Nexus 6 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 7.1.1; Nexus 6 Build/N6F26U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  732,
				Height: 412,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 6P": {
			Name:      "Nexus 6P",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Nexus 6P Build/OPP3.170518.006) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  412,
				Height: 732,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 6P landscape": {
			Name:      "Nexus 6P landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Nexus 6P Build/OPP3.170518.006) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  732,
				Height: 412,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 7": {
			Name:      "Nexus 7",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 7 Build/MOB30X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Safari/537.36",
			Viewport: Viewport{
				Width:  600,
				Height: 960,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nexus 7 landscape": {
			Name:      "Nexus 7 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 7 Build/MOB30X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Safari/537.36",
			Viewport: Viewport{
				Width:  960,
				Height: 600,
			},
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nokia Lumia 520": {
			Name:      "Nokia Lumia 520",
			UserAgent: "Mozilla/5.0 (compatible; MSIE 10.0; Windows Phone 8.0; Trident/6.0; IEMobile/10.0; ARM; Touch; NOKIA; Lumia 520)",
			Viewport: Viewport{
				Width:  320,
				Height: 533,
			},
			DeviceScaleFactor: 1.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nokia Lumia 520 landscape": {
			Name:      "Nokia Lumia 520 landscape",
			UserAgent: "Mozilla/5.0 (compatible; MSIE 10.0; Windows Phone 8.0; Trident/6.0; IEMobile/10.0; ARM; Touch; NOKIA; Lumia 520)",
			Viewport: Viewport{
				Width:  533,
				Height: 320,
			},
			DeviceScaleFactor: 1.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nokia N9": {
			Name:      "Nokia N9",
			UserAgent: "Mozilla/5.0 (MeeGo; NokiaN9) AppleWebKit/534.13 (KHTML, like Gecko) NokiaBrowser/8.5.0 Mobile Safari/534.13",
			Viewport: Viewport{
				Width:  480,
				Height: 854,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Nokia N9 landscape": {
			Name:      "Nokia N9 landscape",
			UserAgent: "Mozilla/5.0 (MeeGo; NokiaN9) AppleWebKit/534.13 (KHTML, like Gecko) NokiaBrowser/8.5.0 Mobile Safari/534.13",
			Viewport: Viewport{
				Width:  854,
				Height: 480,
			},
			DeviceScaleFactor: 1,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Pixel 2": {
			Name:      "Pixel 2",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0; Pixel 2 Build/OPD3.170816.012) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  411,
				Height: 731,
			},
			DeviceScaleFactor: 2.625,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Pixel 2 landscape": {
			Name:      "Pixel 2 landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0; Pixel 2 Build/OPD3.170816.012) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  731,
				Height: 411,
			},
			DeviceScaleFactor: 2.625,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Pixel 2 XL": {
			Name:      "Pixel 2 XL",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Pixel 2 XL Build/OPD1.170816.004) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  411,
				Height: 823,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		"Pixel 2 XL landscape": {
			Name:      "Pixel 2 XL landscape",
			UserAgent: "Mozilla/5.0 (Linux; Android 8.0.0; Pixel 2 XL Build/OPD1.170816.004) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3765.0 Mobile Safari/537.36",
			Viewport: Viewport{
				Width:  823,
				Height: 411,
			},
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
	}
}
