package contracts

// NOTE: This file was automatically generated.

// An instance of PageView represents a generic action on a page like a button
// click. It is also the base type for PageView.
type PageViewData struct {
	Domain
	EventData

	// Request URL with all query string parameters
	Url string `json:"url"`

	// Request duration in format: DD.HH:MM:SS.MMMMMM. For a page view
	// (PageViewData), this is the duration. For a page view with performance
	// information (PageViewPerfData), this is the page load time. Must be less
	// than 1000 days.
	Duration string `json:"duration"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *PageViewData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".PageView"
	} else {
		return "Microsoft.ApplicationInsights.PageView"
	}
}

// Returns the base type when placed within a Data object container.
func (data *PageViewData) BaseType() string {
	return "PageViewData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *PageViewData) Sanitize() []string {
	var warnings []string

	if len(data.Url) > 2048 {
		data.Url = data.Url[:2048]
		warnings = append(warnings, "PageViewData.Url exceeded maximum length of 2048")
	}

	if len(data.Name) > 512 {
		data.Name = data.Name[:512]
		warnings = append(warnings, "PageViewData.Name exceeded maximum length of 512")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "PageViewData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "PageViewData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	if data.Measurements != nil {
		for k, v := range data.Measurements {
			if len(k) > 150 {
				data.Measurements[k[:150]] = v
				delete(data.Measurements, k)
				warnings = append(warnings, "PageViewData.Measurements has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new PageViewData instance with default values set by the schema.
func NewPageViewData() *PageViewData {
	return &PageViewData{
		EventData: EventData{
			Ver: 2,
		},
	}
}
