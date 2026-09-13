package openskynetwork_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/markusheinemann/downline/packages/openskynetwork"
)

const exampleResponse = `{
	"time":1700000000,
	"states":[
		["39de4f","TVF33VV ","France",1700000001,1700000002,19.0467,40.2121,11590.02,false,218.34,307.44,0,null,12085.32,"1673",false,0],
		["50047c","T7AKR20 ","San Marino",1789300433,1789300433,50.8433,23.0877,12496.8,false,230.55,99.5,0,null,13304.52,"1674",false,0]
	]
}`

const exampleResponseEmpty = `{
	"time":1789300433,
	"states":null
}`

func TestClient_ListAllStateVectorsWithoutParameters(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/states/all" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(exampleResponse))
	})

	res, err := c.ListAllStateVectors(context.Background(), openskynetwork.StateVectorOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if res.Time != 1700000000 {
		t.Errorf("expected time to be 1700000000, got %d", res.Time)
	}

	if len(res.States) != 2 {
		t.Errorf("expected 2 states, got %d", len(res.States))
	}

	state := res.States[0]
	if state.Icao24 != "39de4f" {
		t.Errorf("expected icao24 to be 39de4f, got %s", state.Icao24)
	}
	if state.Callsign == nil || *state.Callsign != "TVF33VV " {
		t.Errorf("expected callsign to be TVF33VV , got %s", *state.Callsign)
	}
	if state.OriginCountry != "France" {
		t.Errorf("expected origin country to be France, got %s", state.OriginCountry)
	}
	if state.TimePosition == nil || *state.TimePosition != 1700000001 {
		t.Errorf("expected time position to be 1700000001, got %d", state.TimePosition)
	}
	if state.LastContact != 1700000002 {
		t.Errorf("expected last contact to be 1700000002, got %d", state.LastContact)
	}
	if state.Longitude == nil || *state.Longitude != 19.0467 {
		t.Errorf("expected longitude to be 19.0467, got %f", *state.Longitude)
	}
	if state.Latitude == nil || *state.Latitude != 40.2121 {
		t.Errorf("expected latitude to be 40.2121, got %f", *state.Latitude)
	}
	if state.BaroAltitude == nil || *state.BaroAltitude != 11590.02 {
		t.Errorf("expected baro altitude to be 11590.02, got %f", *state.BaroAltitude)
	}
	if state.OnGround == true {
		t.Errorf("expected on ground to be false, got %t", state.OnGround)
	}
	if state.Velocity == nil || *state.Velocity != 218.34 {
		t.Errorf("expected velocity to be 218.34, got %f", *state.Velocity)
	}
	if state.TrueTrack == nil || *state.TrueTrack != 307.44 {
		t.Errorf("expected true track to be 307.44, got %f", *state.TrueTrack)
	}
	if state.VerticalRate == nil || *state.VerticalRate != 0.0 {
		t.Errorf("expected vertical rate to be 0.0, got %f", *state.VerticalRate)
	}
	if state.Sensors != nil {
		t.Errorf("expected sensors to be nil, got %v", state.Sensors)
	}
	if state.GeoAltitude == nil || *state.GeoAltitude != 12085.32 {
		t.Errorf("expected geo altitude to be 12085.32, got %f", *state.GeoAltitude)
	}
	if state.Squawk == nil || *state.Squawk != "1673" {
		t.Errorf("expected squawk to be 0, got %s", *state.Squawk)
	}
	if state.Spi != false {
		t.Errorf("expected spi to be false, got %t", state.Spi)
	}
	if state.PositionSource != openskynetwork.PositionSourceADSB {
		t.Errorf("expected position source to be ADSB, got %v", state.PositionSource)
	}
}

func TestClient_ListAllStateVectorsWithParameters(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/states/all" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if got := r.URL.Query().Get("time"); got != "1700000000" {
			t.Errorf("time = %s, want 1700000000", got)
		}
		if got := r.URL.Query().Get("icao24"); got != "0000" {
			t.Errorf("icao24 = %s, want 0000", got)
		}
		if got := r.URL.Query().Get("lamax"); got != "10" {
			t.Errorf("lamax = %s, want 10", got)
		}
		if got := r.URL.Query().Get("lamin"); got != "10" {
			t.Errorf("lamin = %s, want 10", got)
		}
		if got := r.URL.Query().Get("lomax"); got != "10" {
			t.Errorf("lomax = %s, want 10", got)
		}
		if got := r.URL.Query().Get("lomin"); got != "10" {
			t.Errorf("lomin = %s, want 10", got)
		}
		if got := r.URL.Query().Get("extended"); got != "1" {
			t.Errorf("extended = %s, want 1", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(exampleResponse))
	})

	res, err := c.ListAllStateVectors(context.Background(), openskynetwork.StateVectorOptions{
		Time:   time.Unix(1700000000, 0),
		Icao24: []string{"0000"},
		BoundingBox: &openskynetwork.BoundingBox{
			LatMin: 10,
			LonMin: 10,
			LatMax: 10,
			LonMax: 10,
		},
		Extended: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if res == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestListAllStateVectorsAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	})

	_, err := c.ListAllStateVectors(context.Background(), openskynetwork.StateVectorOptions{})
	var apiErr *openskynetwork.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status code to be %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestListAllStateVectorsDecoding(t *testing.T) {
	cases := map[string]struct {
		response      string
		expectedError bool
	}{
		"empty array": {
			response:      "[]",
			expectedError: true,
		},
		"value missing": {
			response:      `["39de4f","TVF33VV ","France",1700000001,1700000002,19.0467,40.2121,11590.02,false,218.34,307.44,0,null,12085.32,"1673",false]`,
			expectedError: true,
		},
		"correct number of values": {
			response:      `["39de4f","TVF33VV ","France",1700000001,1700000002,19.0467,40.2121,11590.02,false,218.34,307.44,0,null,12085.32,"1673",false, 0]`,
			expectedError: false,
		},
		"to many values": {
			response:      `["39de4f","TVF33VV ","France",1700000001,1700000002,19.0467,40.2121,11590.02,false,218.34,307.44,0,null,12085.32,"1673",false, 0, "xx"]`,
			expectedError: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/states/all" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(fmt.Sprintf(`{
					"time":1700000000,
					"states":[
						%s
					]
				}`, test.response)))
			})

			_, err := c.ListAllStateVectors(context.Background(), openskynetwork.StateVectorOptions{})
			if err != nil && !test.expectedError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && test.expectedError {
				t.Fatal("expected error")
			}
		})
	}
}
