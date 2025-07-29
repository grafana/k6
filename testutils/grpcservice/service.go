package grpcservice

import (
	"context"
)

// RouteGuideServer implements the RouteGuide service
type routeGuideServer struct {
	UnimplementedRouteGuideServer
	features []*Feature
}

// NewRouteGuideServer creates a new RouteGuide server
func NewRouteGuideServer(features ...*Feature) RouteGuideServer {
	return &routeGuideServer{features: features}
}

// GetFeature returns a feature at a given point
func (s *routeGuideServer) GetFeature(ctx context.Context, point *Point) (*Feature, error) {
	for _, feature := range s.features {
		if feature.Location.Latitude == point.Latitude && feature.Location.Longitude == point.Longitude {
			return feature, nil
		}
	}
	return &Feature{Location: point}, nil
}

// LoadFeatures loads test features
func LoadFeatures(dataPath string) []*Feature {
	return []*Feature{
		{
			Name: "Test Location",
			Location: &Point{
				Latitude:  409146138,
				Longitude: -746188906,
			},
		},
	}
} 