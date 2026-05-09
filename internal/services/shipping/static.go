package shipping

import (
	"sort"
	"strings"
)

// Static dataset & rate calculator. Used when no external rate API
// (Biteship/RajaOngkir/etc.) is configured. Trade-off: tariffs are
// estimates, not real-time. Update the rateTable below to your liking.

type staticArea struct {
	ID       string
	Province string
	City     string
	District string
	Postal   string
	Zone     int // 1..6
}

// Indonesian zones (loosely matching JNE pricing logic).
//   1 — Jabodetabek
//   2 — Jawa selain Jabodetabek
//   3 — Bali, Sumatera, NTB
//   4 — Kalimantan, Sulawesi
//   5 — NTT, Maluku
//   6 — Papua
var staticAreas = []staticArea{
	// Zone 1 — Jabodetabek
	{"AREA-JKTP", "DKI Jakarta", "Jakarta Pusat", "", "10110", 1},
	{"AREA-JKTU", "DKI Jakarta", "Jakarta Utara", "", "14110", 1},
	{"AREA-JKTB", "DKI Jakarta", "Jakarta Barat", "", "11110", 1},
	{"AREA-JKTS", "DKI Jakarta", "Jakarta Selatan", "", "12110", 1},
	{"AREA-JKTT", "DKI Jakarta", "Jakarta Timur", "", "13220", 1},
	{"AREA-BOGOR", "Jawa Barat", "Kota Bogor", "", "16111", 1},
	{"AREA-KAB-BOGOR", "Jawa Barat", "Kab. Bogor", "", "16911", 1},
	{"AREA-DEPOK", "Jawa Barat", "Kota Depok", "", "16411", 1},
	{"AREA-TANGERANG", "Banten", "Kota Tangerang", "", "15111", 1},
	{"AREA-TANGSEL", "Banten", "Tangerang Selatan", "", "15311", 1},
	{"AREA-KAB-TNG", "Banten", "Kab. Tangerang", "", "15710", 1},
	{"AREA-BEKASI", "Jawa Barat", "Kota Bekasi", "", "17111", 1},
	{"AREA-KAB-BKS", "Jawa Barat", "Kab. Bekasi", "", "17510", 1},
	{"AREA-KARAWANG", "Jawa Barat", "Kab. Karawang", "", "41311", 1},
	{"AREA-CIKARANG", "Jawa Barat", "Cikarang", "Kab. Bekasi", "17530", 1},

	// Zone 2 — Jawa lain
	{"AREA-BANDUNG", "Jawa Barat", "Kota Bandung", "", "40111", 2},
	{"AREA-KAB-BDG", "Jawa Barat", "Kab. Bandung", "", "40911", 2},
	{"AREA-CIMAHI", "Jawa Barat", "Kota Cimahi", "", "40511", 2},
	{"AREA-CIREBON", "Jawa Barat", "Kota Cirebon", "", "45111", 2},
	{"AREA-SUKABUMI", "Jawa Barat", "Kota Sukabumi", "", "43111", 2},
	{"AREA-TASIK", "Jawa Barat", "Kota Tasikmalaya", "", "46111", 2},
	{"AREA-GARUT", "Jawa Barat", "Kab. Garut", "", "44111", 2},
	{"AREA-SEMARANG", "Jawa Tengah", "Kota Semarang", "", "50111", 2},
	{"AREA-SOLO", "Jawa Tengah", "Kota Surakarta", "", "57111", 2},
	{"AREA-MAGELANG", "Jawa Tengah", "Kota Magelang", "", "56111", 2},
	{"AREA-PEKALONGAN", "Jawa Tengah", "Kota Pekalongan", "", "51111", 2},
	{"AREA-PURWOKERTO", "Jawa Tengah", "Kab. Banyumas", "", "53111", 2},
	{"AREA-TEGAL", "Jawa Tengah", "Kota Tegal", "", "52111", 2},
	{"AREA-YOGYAKARTA", "DI Yogyakarta", "Kota Yogyakarta", "", "55111", 2},
	{"AREA-SLEMAN", "DI Yogyakarta", "Kab. Sleman", "", "55511", 2},
	{"AREA-BANTUL", "DI Yogyakarta", "Kab. Bantul", "", "55711", 2},
	{"AREA-SURABAYA", "Jawa Timur", "Kota Surabaya", "", "60111", 2},
	{"AREA-MALANG", "Jawa Timur", "Kota Malang", "", "65111", 2},
	{"AREA-KAB-MLG", "Jawa Timur", "Kab. Malang", "", "65211", 2},
	{"AREA-SIDOARJO", "Jawa Timur", "Kab. Sidoarjo", "", "61211", 2},
	{"AREA-GRESIK", "Jawa Timur", "Kab. Gresik", "", "61111", 2},
	{"AREA-KEDIRI", "Jawa Timur", "Kota Kediri", "", "64111", 2},
	{"AREA-MADIUN", "Jawa Timur", "Kota Madiun", "", "63111", 2},
	{"AREA-JEMBER", "Jawa Timur", "Kab. Jember", "", "68111", 2},
	{"AREA-BANYUWANGI", "Jawa Timur", "Kab. Banyuwangi", "", "68411", 2},
	{"AREA-CILEGON", "Banten", "Kota Cilegon", "", "42411", 2},
	{"AREA-SERANG", "Banten", "Kota Serang", "", "42111", 2},

	// Zone 3 — Bali, Sumatera, NTB
	{"AREA-DENPASAR", "Bali", "Kota Denpasar", "", "80111", 3},
	{"AREA-BADUNG", "Bali", "Kab. Badung", "", "80351", 3},
	{"AREA-GIANYAR", "Bali", "Kab. Gianyar", "", "80511", 3},
	{"AREA-MEDAN", "Sumatera Utara", "Kota Medan", "", "20111", 3},
	{"AREA-DELI-SERDANG", "Sumatera Utara", "Kab. Deli Serdang", "", "20911", 3},
	{"AREA-PALEMBANG", "Sumatera Selatan", "Kota Palembang", "", "30111", 3},
	{"AREA-PEKANBARU", "Riau", "Kota Pekanbaru", "", "28111", 3},
	{"AREA-PADANG", "Sumatera Barat", "Kota Padang", "", "25111", 3},
	{"AREA-LAMPUNG", "Lampung", "Bandar Lampung", "", "35111", 3},
	{"AREA-BATAM", "Kepulauan Riau", "Kota Batam", "", "29111", 3},
	{"AREA-ACEH", "Aceh", "Kota Banda Aceh", "", "23111", 3},
	{"AREA-JAMBI", "Jambi", "Kota Jambi", "", "36111", 3},
	{"AREA-BENGKULU", "Bengkulu", "Kota Bengkulu", "", "38111", 3},
	{"AREA-MATARAM", "Nusa Tenggara Barat", "Kota Mataram", "", "83111", 3},
	{"AREA-LOMBOK-T", "Nusa Tenggara Barat", "Kab. Lombok Timur", "", "83611", 3},

	// Zone 4 — Kalimantan, Sulawesi
	{"AREA-BALIKPAPAN", "Kalimantan Timur", "Kota Balikpapan", "", "76111", 4},
	{"AREA-SAMARINDA", "Kalimantan Timur", "Kota Samarinda", "", "75111", 4},
	{"AREA-PONTIANAK", "Kalimantan Barat", "Kota Pontianak", "", "78111", 4},
	{"AREA-BANJARMASIN", "Kalimantan Selatan", "Kota Banjarmasin", "", "70111", 4},
	{"AREA-PALANGKARAYA", "Kalimantan Tengah", "Palangka Raya", "", "73111", 4},
	{"AREA-MAKASSAR", "Sulawesi Selatan", "Kota Makassar", "", "90111", 4},
	{"AREA-MANADO", "Sulawesi Utara", "Kota Manado", "", "95111", 4},
	{"AREA-PALU", "Sulawesi Tengah", "Kota Palu", "", "94111", 4},
	{"AREA-KENDARI", "Sulawesi Tenggara", "Kota Kendari", "", "93111", 4},
	{"AREA-GORONTALO", "Gorontalo", "Kota Gorontalo", "", "96111", 4},

	// Zone 5 — NTT, Maluku
	{"AREA-KUPANG", "Nusa Tenggara Timur", "Kota Kupang", "", "85111", 5},
	{"AREA-AMBON", "Maluku", "Kota Ambon", "", "97111", 5},
	{"AREA-TERNATE", "Maluku Utara", "Kota Ternate", "", "97711", 5},

	// Zone 6 — Papua
	{"AREA-JAYAPURA", "Papua", "Kota Jayapura", "", "99111", 6},
	{"AREA-MANOKWARI", "Papua Barat", "Kota Manokwari", "", "98311", 6},
	{"AREA-SORONG", "Papua Barat", "Kota Sorong", "", "98411", 6},
	{"AREA-MERAUKE", "Papua Selatan", "Kab. Merauke", "", "99611", 6},
}

// Rate per kg, in IDR. Index 0..5 corresponds to zones 1..6.
// Zero means "service not available for that zone".
var rateTable = map[string][6]float64{
	// reguler 2-4 hari
	"jne_reg":      {9000, 14000, 22000, 32000, 45000, 65000},
	"jnt_reg":      {9000, 13500, 21000, 31000, 43000, 62000},
	"sicepat_reg":  {8000, 12500, 20000, 30000, 42000, 60000},
	"anteraja_reg": {7500, 12000, 20000, 30000, 42000, 60000},
	"pos_kilat":    {10000, 11000, 17000, 25000, 36000, 52000},
	// next-day / Yes (zone 1-2 saja)
	"jne_yes": {18000, 25000, 0, 0, 0, 0},
}

type courierMeta struct {
	Code        string
	CourierName string
	ServiceCode string
	ServiceName string
	ETD         [6]string
}

var couriers = []courierMeta{
	{"jne", "JNE", "REG", "Reguler", [6]string{"1-2 hari", "2-3 hari", "3-5 hari", "4-6 hari", "5-7 hari", "6-9 hari"}},
	{"jnt", "J&T Express", "EZ", "Reguler", [6]string{"1-2 hari", "2-3 hari", "3-5 hari", "4-6 hari", "5-7 hari", "6-9 hari"}},
	{"sicepat", "SiCepat", "REG", "Reguler", [6]string{"1-2 hari", "2-3 hari", "3-5 hari", "4-6 hari", "5-7 hari", "6-9 hari"}},
	{"anteraja", "AnterAja", "REG", "Reguler", [6]string{"1-2 hari", "2-3 hari", "3-5 hari", "4-6 hari", "5-7 hari", "6-9 hari"}},
	{"pos", "POS Indonesia", "KILAT", "Kilat Khusus", [6]string{"1-3 hari", "2-4 hari", "3-5 hari", "4-7 hari", "5-8 hari", "7-12 hari"}},
	{"jne", "JNE", "YES", "Yakin Esok Sampai", [6]string{"1 hari", "1-2 hari", "", "", "", ""}},
}

// rateKey maps courier code + service to rateTable key.
var courierRateKey = map[string]string{
	"jne|REG":      "jne_reg",
	"jne|YES":      "jne_yes",
	"jnt|EZ":       "jnt_reg",
	"sicepat|REG":  "sicepat_reg",
	"anteraja|REG": "anteraja_reg",
	"pos|KILAT":    "pos_kilat",
}

// searchStaticAreas returns areas matching q (case-insensitive substring on
// city + province + postal). Limited to top N by name.
func searchStaticAreas(q string) []Area {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	type scored struct {
		a     staticArea
		score int
	}
	var hits []scored
	for _, sa := range staticAreas {
		c := strings.ToLower(sa.City)
		p := strings.ToLower(sa.Province)
		d := strings.ToLower(sa.District)
		s := 0
		switch {
		case strings.HasPrefix(c, q):
			s = 100
		case strings.Contains(c, q):
			s = 80
		case strings.HasPrefix(p, q):
			s = 60
		case strings.Contains(p, q):
			s = 40
		case d != "" && strings.Contains(d, q):
			s = 30
		case strings.HasPrefix(sa.Postal, q):
			s = 20
		}
		if s > 0 {
			hits = append(hits, scored{sa, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > 25 {
		hits = hits[:25]
	}
	out := make([]Area, 0, len(hits))
	for _, h := range hits {
		out = append(out, Area{
			ID:          h.a.ID,
			Name:        h.a.City + ", " + h.a.Province,
			CountryName: "Indonesia",
			AdminLevel1: h.a.Province,
			AdminLevel2: h.a.City,
			AdminLevel3: h.a.District,
			PostalCode:  h.a.Postal,
		})
	}
	return out
}

// findStaticArea looks up by ID or postal code (whichever matches).
func findStaticArea(areaID, postal string) (staticArea, bool) {
	for _, sa := range staticAreas {
		if (areaID != "" && sa.ID == areaID) || (postal != "" && sa.Postal == postal) {
			return sa, true
		}
	}
	// fallback: postal prefix match (first 2 digits identify region group)
	if len(postal) >= 2 {
		prefix := postal[:2]
		for _, sa := range staticAreas {
			if strings.HasPrefix(sa.Postal, prefix) {
				return sa, true
			}
		}
	}
	return staticArea{}, false
}

// staticRates computes courier options for a destination + total weight.
// originPostal is hint; we infer origin zone from it (or assume zone 1).
func staticRates(originPostal string, req RateRequest) []Rate {
	// Determine destination zone
	var destZone int
	if dest, ok := findStaticArea(req.DestAreaID, req.DestPostalCode); ok {
		destZone = dest.Zone
	} else {
		// unknown destination → default to zone 3 (a sensible middle ground)
		destZone = 3
	}
	if destZone < 1 {
		destZone = 1
	}
	if destZone > 6 {
		destZone = 6
	}
	// Total weight: sum of all items in grams, min 1kg
	totalGrams := 0
	for _, it := range req.Items {
		w := it.Weight
		if w <= 0 {
			w = 500
		}
		totalGrams += w * it.Quantity
	}
	if totalGrams < 1000 {
		totalGrams = 1000
	}
	kg := (totalGrams + 999) / 1000

	results := make([]Rate, 0, len(couriers))
	for _, c := range couriers {
		key := c.Code + "|" + c.ServiceCode
		rateKey, ok := courierRateKey[key]
		if !ok {
			continue
		}
		ratesArr := rateTable[rateKey]
		ratePerKg := ratesArr[destZone-1]
		if ratePerKg <= 0 {
			continue
		}
		etd := c.ETD[destZone-1]
		results = append(results, Rate{
			CourierCode:    c.Code,
			CourierName:    c.CourierName,
			CourierService: c.ServiceCode,
			ServiceName:    c.ServiceName,
			Price:          ratePerKg * float64(kg),
			Type:           "ground",
			Duration:       etd,
			ETD:            etd,
		})
	}
	return results
}
