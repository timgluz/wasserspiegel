package station

import "github.com/gosimple/slug"

type StationCollection struct {
	Stations []Station `json:"stations"`
}

type StationList []Station

type Station struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Water string `json:"water"`

	Location    Location     `json:"location"`
	ExternalIDs []ExternalID `json:"external_ids,omitempty"`
}

func (s Station) GetExternalID(name string) (string, bool) {
	for _, id := range s.ExternalIDs {
		if id.Name == name {
			return id.ID, true
		}
	}
	return "", false
}

type ExternalID struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Location struct {
	KM        float64 `json:"km"` // distance from the river source
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func NewStationID(waterName, stationName string) string {
	return slug.Make(waterName + "-" + stationName)
}
