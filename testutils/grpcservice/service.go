// Package grpcservice provides a gRPC test service for k6 tests.
package grpcservice

import (
	"context"
	"encoding/json"
	"os"
)

// LoadFeatures loads test features from a JSON file.
func LoadFeatures(jsonFile string) []*Feature {
	if jsonFile == "" {
		return []*Feature{
			{
				Name: "Test Feature",
				Location: &Point{
					Latitude:  409146138,
					Longitude: -746188906,
				},
			},
		}
	}

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil
	}

	var features []*Feature
	if err := json.Unmarshal(data, &features); err != nil {
		return nil
	}

	return features
}

type routeGuideServer struct {
	UnimplementedRouteGuideServer
	features []*Feature
}

// NewRouteGuideServer creates a new RouteGuide server with the given features.
func NewRouteGuideServer(features ...*Feature) RouteGuideServer {
	return &routeGuideServer{features: features}
}

func (s *routeGuideServer) GetFeature(ctx context.Context, point *Point) (*Feature, error) {
	for _, feature := range s.features {
		if feature.Location.Latitude == point.Latitude && feature.Location.Longitude == point.Longitude {
			return feature, nil
		}
	}
	return &Feature{Location: point}, nil
}