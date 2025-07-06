package station

import (
	"testing"

	"github.com/mailru/easyjson"
)

func TestUnmarshalStationList_HappyCase(t *testing.T) {
	data := []byte(`[{"uuid":"1234","longname":"Station One","shortname":"S1","km":10.5,"latitude":52.5200,"longitude":13.4050,"water":{"longname":"Water One","shortname":"W1"}},{"uuid":"5678","longname":"Station Two","shortname":"S2","km":20.0,"latitude":48.8566,"longitude":2.3522,"water":{"longname":"Water Two","shortname":"W2"}}]`)

	var stations StationList
	if err := easyjson.Unmarshal(data, &stations); err != nil {
		t.Fatalf("Failed to unmarshal StationList: %v", err)
	}

	if len(stations) != 2 {
		t.Errorf("Expected 2 stations, got %d", len(stations))
	}
}
