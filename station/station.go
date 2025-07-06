package station

type StationCollection struct {
	Stations []Station `json:"stations"`
}

type StationList []Station

type Station struct {
	UUID      string       `json:"uuid"`
	Name      string       `json:"longname"`
	ShortName string       `json:"shortname"`
	KM        float64      `json:"km"`
	Latitude  float64      `json:"latitude"`
	Longitude float64      `json:"longitude"`
	Water     StationWater `json:"water"`
}

type StationWater struct {
	Name      string `json:"longname"`
	ShortName string `json:"shortname"`
}
