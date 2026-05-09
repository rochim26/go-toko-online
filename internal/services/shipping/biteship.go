package shipping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tokoonline/app/internal/services/settings"
)

type Biteship struct {
	settings *settings.Store
	hc       *http.Client
}

func New(s *settings.Store) *Biteship {
	return &Biteship{
		settings: s,
		hc:       &http.Client{Timeout: 10 * time.Second},
	}
}

type Area struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CountryName   string `json:"country_name"`
	AdminLevel1   string `json:"administrative_division_level_1_name"`
	AdminLevel2   string `json:"administrative_division_level_2_name"`
	AdminLevel3   string `json:"administrative_division_level_3_name"`
	PostalCode    string `json:"-"`
}

// Biteship returns postal_code as a JSON number (e.g. 13220) for some areas
// but as a string for others. Accept both.
func (a *Area) UnmarshalJSON(b []byte) error {
	type alias Area
	raw := struct {
		*alias
		PostalCode json.RawMessage `json:"postal_code"`
	}{alias: (*alias)(a)}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw.PostalCode) == 0 || string(raw.PostalCode) == "null" {
		a.PostalCode = ""
		return nil
	}
	s := string(raw.PostalCode)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		a.PostalCode = s[1 : len(s)-1]
	} else {
		a.PostalCode = s
	}
	return nil
}

func (b *Biteship) SearchArea(ctx context.Context, q string) ([]Area, error) {
	cfg := b.settings.Biteship()
	if cfg.APIKey == "" {
		return searchStaticAreas(q), nil
	}
	u := fmt.Sprintf("https://api.biteship.com/v1/maps/areas?countries=ID&input=%s&type=single", urlEscape(q))
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", cfg.APIKey)
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("biteship area: %s: %s", resp.Status, string(body))
	}
	var out struct {
		Areas []Area `json:"areas"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Areas, nil
}

type RateItem struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Quantity int     `json:"quantity"`
	Weight   int     `json:"weight"`
}

type RateRequest struct {
	OriginAreaID      string     `json:"origin_area_id,omitempty"`
	OriginPostalCode  string     `json:"origin_postal_code,omitempty"`
	DestAreaID        string     `json:"destination_area_id,omitempty"`
	DestPostalCode    string     `json:"destination_postal_code,omitempty"`
	Couriers          string     `json:"couriers"`
	Items             []RateItem `json:"items"`
}

type Rate struct {
	CourierCode    string  `json:"courier_code"`
	CourierName    string  `json:"courier_name"`
	CourierService string  `json:"courier_service_code"`
	ServiceName    string  `json:"courier_service_name"`
	Price          float64 `json:"price"`
	Type           string  `json:"type"`
	Duration       string  `json:"duration"`
	ETD            string  `json:"shipment_duration_range"`
}

func (b *Biteship) Rates(ctx context.Context, req RateRequest) ([]Rate, error) {
	cfg := b.settings.Biteship()
	if cfg.APIKey == "" {
		return staticRates(cfg.OriginPostalCode, req), nil
	}
	if req.OriginAreaID == "" {
		req.OriginAreaID = cfg.OriginAreaID
	}
	if req.OriginPostalCode == "" {
		req.OriginPostalCode = cfg.OriginPostalCode
	}
	if req.Couriers == "" {
		req.Couriers = strings.Join(cfg.Couriers, ",")
	}
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", "https://api.biteship.com/v1/rates/couriers", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := b.hc.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("biteship rates: %s: %s", resp.Status, string(rb))
	}
	var out struct {
		Pricing []struct {
			CourierCode    string  `json:"courier_code"`
			CourierName    string  `json:"courier_name"`
			CourierService string  `json:"courier_service_code"`
			ServiceName    string  `json:"courier_service_name"`
			Price          float64 `json:"price"`
			Type           string  `json:"type"`
			Duration       string  `json:"duration"`
			ETD            string  `json:"shipment_duration_range"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, err
	}
	rates := make([]Rate, 0, len(out.Pricing))
	for _, p := range out.Pricing {
		rates = append(rates, Rate(p))
	}
	return rates, nil
}

func urlEscape(s string) string {
	r := strings.NewReplacer(" ", "%20", ",", "%2C")
	return r.Replace(s)
}
