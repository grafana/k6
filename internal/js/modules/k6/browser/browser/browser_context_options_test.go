package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestBrowserContextOptionsPermissions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	opts, err := parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		Permissions []any `js:"permissions"`
	}{
		Permissions: []any{"camera", "microphone"},
	})))
	assert.NoError(t, err)
	assert.Len(t, opts.Permissions, 2)
	assert.Equal(t, opts.Permissions, []string{"camera", "microphone"})
}

func TestBrowserContextSetGeolocation(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	opts, err := parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		GeoLocation *common.Geolocation `js:"geolocation"`
	}{
		GeoLocation: &common.Geolocation{
			Latitude:  1.0,
			Longitude: 2.0,
			Accuracy:  3.0,
		},
	})))
	assert.NoError(t, err)
	assert.NotNil(t, opts)
	assert.Equal(t, 1.0, opts.Geolocation.Latitude)
	assert.Equal(t, 2.0, opts.Geolocation.Longitude)
	assert.Equal(t, 3.0, opts.Geolocation.Accuracy)
}

func TestBrowserContextDefaultOptions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	defaults := common.DefaultBrowserContextOptions()

	// gets the default options by default
	opts, err := parseBrowserContextOptions(vu.Runtime(), nil)
	require.NoError(t, err)
	assert.Equal(t, defaults, opts)

	// merges with the default options
	opts, err = parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		DeviceScaleFactor float64 `js:"deviceScaleFactor"` // just to test a different field
	}{
		DeviceScaleFactor: defaults.DeviceScaleFactor + 1,
	})))
	require.NoError(t, err)
	assert.NotEqual(t, defaults.DeviceScaleFactor, opts.DeviceScaleFactor)
	assert.Equal(t, defaults.Locale, opts.Locale) // should remain as default
}

func TestBrowserContextAllOptions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)
	opts, err := vu.Runtime().RunString(`const opts = {
			acceptDownloads: true,
			downloadsPath: '/tmp',
			bypassCSP: true,
			colorScheme: 'dark',
			deviceScaleFactor: 1,
			extraHTTPHeaders: {
				'X-Header': 'value',
			},
			geolocation: { latitude: 51.509865, longitude: -0.118092, accuracy: 1 },
			hasTouch: true,
			httpCredentials: { username: 'admin', password: 'password' },
			ignoreHTTPSErrors: true,
			isMobile: true,
			javaScriptEnabled: true,
			locale: 'fr-FR',
			offline: true,
			permissions: ['camera', 'microphone'],
			reducedMotion: 'no-preference',
			screen: { width: 800, height: 600 },
			timezoneID: 'Europe/Paris',
			userAgent: 'my agent',
			viewport: { width: 800, height: 600 },
		};
		opts;
	`)
	require.NoError(t, err)

	parsedOpts, err := parseBrowserContextOptions(vu.Runtime(), opts)
	require.NoError(t, err)

	assert.Equal(t, &common.BrowserContextOptions{
		AcceptDownloads:   true,
		DownloadsPath:     "/tmp",
		BypassCSP:         true,
		ColorScheme:       common.ColorSchemeDark,
		DeviceScaleFactor: 1,
		ExtraHTTPHeaders: map[string]string{
			"X-Header": "value",
		},
		Geolocation: &common.Geolocation{
			Latitude:  51.509865,
			Longitude: -0.118092,
			Accuracy:  1,
		},
		HasTouch: true,
		HTTPCredentials: common.Credentials{
			Username: "admin",
			Password: "password",
		},
		IgnoreHTTPSErrors: true,
		IsMobile:          true,
		JavaScriptEnabled: true,
		Locale:            "fr-FR",
		Offline:           true,
		Permissions:       []string{"camera", "microphone"},
		ReducedMotion:     common.ReducedMotionNoPreference,
		Screen: common.Screen{
			Width:  800,
			Height: 600,
		},
		TimezoneID: "Europe/Paris",
		UserAgent:  "my agent",
		Viewport: common.Viewport{
			Width:  800,
			Height: 600,
		},
	}, parsedOpts)
}
