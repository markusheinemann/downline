package openskynetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AircraftCategory is the emitter category reported by the transponder.
// It is only filled in when the request sets StateVectorOptions.Extended.
type AircraftCategory int

const (
	AircraftCategoryNoInformation AircraftCategory = iota
	AircraftCategoryNoADSBEmitterCategory
	AircraftCategoryLight
	AircraftCategorySmall
	AircraftCategoryLarge
	AircraftCategoryHighVortexLarge
	AircraftCategoryHeavy
	AircraftCategoryHighPerformance
	AircraftCategoryRotorcraft
	AircraftCategoryGlider
	AircraftCategoryLighterThanAir
	AircraftCategoryParachutist
	AircraftCategoryUltralight
	AircraftCategoryReserved
	AircraftCategoryUnmannedAerialVehicle
	AircraftCategorySpaceVehicle
	AircraftCategorySurfaceVehicleEmergency
	AircraftCategorySurfaceVehicleService
	AircraftCategoryPointObstacle
	AircraftCategoryClusterObstacle
	AircraftCategoryLineObstacle
)

// PositionSource is the system that produced the position of a StateVector.
type PositionSource int

const (
	PositionSourceADSB PositionSource = iota
	PositionSourceASTERIX
	PositionSourceMLAT
	PositionSourceFLARM
)

// StateVector is the state of one aircraft at a point in time.
// Pointer fields are nil when OpenSky has no value for them.
type StateVector struct {
	// Icao24 is the transponder address as a lowercase hex string, such as "3c6444".
	Icao24 string `json:"icao24"`
	// Callsign can contain trailing spaces. Trim it before comparing.
	Callsign *string `json:"callsign"`
	// OriginCountry is derived from the Icao24 address, not from the flight route.
	OriginCountry string `json:"origin_country"`
	// TimePosition is the Unix time in seconds of the last position update.
	// It is nil if no position was received in the last 15 seconds.
	TimePosition *int64 `json:"time_position"`
	// LastContact is the Unix time in seconds of the last message of any kind.
	LastContact int64 `json:"last_contact"`
	// Longitude is in WGS84 decimal degrees.
	Longitude *float64 `json:"longitude"`
	// Latitude is in WGS84 decimal degrees.
	Latitude *float64 `json:"latitude"`
	// BaroAltitude is the barometric altitude in meters.
	BaroAltitude *float64 `json:"baro_altitude"`
	// OnGround is true if the position came from a surface position report.
	OnGround bool `json:"on_ground"`
	// Velocity is the speed over ground in meters per second.
	Velocity *float64 `json:"velocity"`
	// TrueTrack is the direction of travel in degrees clockwise from north.
	TrueTrack *float64 `json:"true_track"`
	// VerticalRate is in meters per second. Positive values mean climbing.
	VerticalRate *float64 `json:"vertical_rate"`
	// Sensors lists the receivers that contributed to this state. It is only set
	// by requests that filter by sensor, so it is nil for ListAllStateVectors.
	Sensors []int `json:"sensors"`
	// GeoAltitude is the geometric altitude in meters.
	GeoAltitude *float64 `json:"geo_altitude"`
	// Squawk is the transponder code.
	Squawk *string `json:"squawk"`
	// Spi reports whether the special purpose indicator is set.
	Spi            bool           `json:"spi"`
	PositionSource PositionSource `json:"position_source"`
	// Category is only set when the request used StateVectorOptions.Extended.
	Category AircraftCategory `json:"category"`
}

// StateVectorsResponse is the result of ListAllStateVectors.
type StateVectorsResponse struct {
	// Time is the Unix time in seconds the states belong to. Each state
	// reflects the aircraft during the second before Time.
	Time int64 `json:"time"`
	// States can be nil when no aircraft match the options.
	States []StateVector `json:"states"`
}

// StateVectorOptions filters the states returned by ListAllStateVectors.
// The zero value requests all aircraft at the current time.
type StateVectorOptions struct {
	// Time requests past states instead of the latest ones. Authenticated
	// clients can go back up to one hour. Anonymous requests ignore it.
	Time time.Time
	// Icao24 limits the result to these transponder addresses. Case does not matter.
	Icao24 []string
	// BoundingBox limits the result to an area.
	BoundingBox *BoundingBox
	// Extended adds the Category field to each state.
	Extended bool
}

// BoundingBox is an area given in WGS84 decimal degrees.
type BoundingBox struct {
	LatMin float64
	LonMin float64
	LatMax float64
	LonMax float64
}

func (o StateVectorOptions) query() url.Values {
	q := url.Values{}
	if !o.Time.IsZero() {
		q.Set("time", strconv.FormatInt(o.Time.Unix(), 10))
	}

	for _, icao := range o.Icao24 {
		q.Add("icao24", strings.ToLower(icao))
	}

	if b := o.BoundingBox; b != nil {
		q.Set("lamin", strconv.FormatFloat(b.LatMin, 'f', -1, 64))
		q.Set("lomin", strconv.FormatFloat(b.LonMin, 'f', -1, 64))
		q.Set("lamax", strconv.FormatFloat(b.LatMax, 'f', -1, 64))
		q.Set("lomax", strconv.FormatFloat(b.LonMax, 'f', -1, 64))
	}

	if o.Extended {
		q.Set("extended", "1")
	}

	return q
}

// UnmarshalJSON decodes a state from the array format the API returns.
// It does not accept the object format that json.Marshal produces.
func (s *StateVector) UnmarshalJSON(b []byte) error {
	*s = StateVector{}

	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("state vector: %w", err)
	}
	if len(raw) < 17 {
		return fmt.Errorf("state vector: got %d fields, want at least 17", len(raw))
	}

	fields := []any{
		0:  &s.Icao24,
		1:  &s.Callsign,
		2:  &s.OriginCountry,
		3:  &s.TimePosition,
		4:  &s.LastContact,
		5:  &s.Longitude,
		6:  &s.Latitude,
		7:  &s.BaroAltitude,
		8:  &s.OnGround,
		9:  &s.Velocity,
		10: &s.TrueTrack,
		11: &s.VerticalRate,
		12: &s.Sensors,
		13: &s.GeoAltitude,
		14: &s.Squawk,
		15: &s.Spi,
		16: &s.PositionSource,
		17: &s.Category,
	}

	for i := 0; i < min(len(raw), len(fields)); i++ {
		if err := json.Unmarshal(raw[i], fields[i]); err != nil {
			return fmt.Errorf("state vector index %d: %w", i, err)
		}
	}
	return nil
}

// ListAllStateVectors fetches state vectors from GET /states/all.
// A response with a status code outside the 2xx range returns an *APIError.
func (c *Client) ListAllStateVectors(ctx context.Context, options StateVectorOptions) (*StateVectorsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/states/all", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = options.query().Encode()

	var res StateVectorsResponse

	if _, err := c.do(req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
