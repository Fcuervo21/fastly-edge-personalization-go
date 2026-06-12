package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/fastly/compute-sdk-go/device"
	"github.com/fastly/compute-sdk-go/fsthttp"
	"github.com/fastly/compute-sdk-go/geo"
)

type Profile struct {
	Geo    GeoInfo    `json:"geo"`
	Device DeviceInfo `json:"device"`
	Meta   MetaInfo   `json:"meta"`
}

type GeoInfo struct {
	City        string   `json:"city"`
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	Continent   string   `json:"continent"`
	Region      string   `json:"region"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	PostalCode  string   `json:"postal_code"`
	UTCOffset   *int     `json:"utc_offset"`
	AsName      string   `json:"as_name"`
	AsNumber    *int     `json:"as_number"`
	ConnSpeed   string   `json:"conn_speed"`
	ConnType    string   `json:"conn_type"`
}

type DeviceInfo struct {
	Type          string `json:"type"`
	Brand         string `json:"brand"`
	Model         string `json:"model"`
	IsMobile      bool   `json:"is_mobile"`
	IsDesktop     bool   `json:"is_desktop"`
	IsTablet      bool   `json:"is_tablet"`
	IsTouchscreen bool   `json:"is_touchscreen"`
	IsBot         bool   `json:"is_bot"`
}

type MetaInfo struct {
	ClientIP  string `json:"client_ip"`
	UserAgent string `json:"user_agent"`
	RequestID string `json:"request_id"`
}

func main() {
	fsthttp.ServeFunc(func(ctx context.Context, w fsthttp.ResponseWriter, r *fsthttp.Request) {
		switch r.URL.Path {
		case "/_edge/healthcheck":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(fsthttp.StatusOK)
			fmt.Fprint(w, "ok")
		case "/api/profile":
			profile := buildProfile(r)
			body, _ := json.MarshalIndent(profile, "", "  ")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Edge-Personalization", "true")
			w.WriteHeader(fsthttp.StatusOK)
			w.Write(body)
		default:
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(fsthttp.StatusNotFound)
			fmt.Fprint(w, "Not Found\n")
		}
	})
}

func buildProfile(r *fsthttp.Request) Profile {
	userAgent := r.Header.Get("User-Agent")
	clientIP := r.Header.Get("Fastly-Client-IP")
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}

	requestID := ""
	if meta, err := r.FastlyMeta(); err == nil {
		requestID = meta.RequestID
	}

	geoInfo := GeoInfo{
		City:        "Unknown",
		CountryCode: "XX",
		CountryName: "Unknown",
		Continent:   "XX",
		Region:      "Unknown",
		PostalCode:  "N/A",
		AsName:      "Unknown",
		ConnSpeed:   "Unknown",
		ConnType:    "Unknown",
	}

	if ip := net.ParseIP(clientIP); ip != nil {
		if g, err := geo.Lookup(ip); err == nil {
			geoInfo.City = stringOrDefault(g.City, "Unknown")
			geoInfo.CountryCode = stringOrDefault(g.CountryCode, "XX")
			geoInfo.CountryName = stringOrDefault(g.CountryName, "Unknown")
			geoInfo.Continent = stringOrDefault(g.ContinentCode, "XX")
			geoInfo.Region = stringOrDefault(g.Region, "Unknown")
			geoInfo.PostalCode = stringOrDefault(g.PostalCode, "N/A")
			geoInfo.AsName = stringOrDefault(g.AsName, "Unknown")
			geoInfo.ConnSpeed = stringOrDefault(g.ConnSpeed, "Unknown")
			geoInfo.ConnType = stringOrDefault(g.ConnType, "Unknown")

			lat := g.Latitude
			lng := g.Longitude
			geoInfo.Latitude = &lat
			geoInfo.Longitude = &lng

			offset := g.UTCOffset
			geoInfo.UTCOffset = &offset

			asNum := g.AsNumber
			geoInfo.AsNumber = &asNum
		}
	}

	deviceInfo := parseUserAgent(userAgent)
	if d, err := device.Lookup(userAgent); err == nil {
		hwType := d.HWType()
		if hwType != "" && hwType != "Unknown" {
			deviceInfo = DeviceInfo{
				Type:          hwType,
				Brand:         stringOrDefault(d.Brand(), "Unknown"),
				Model:         stringOrDefault(d.Model(), "Unknown"),
				IsMobile:      d.IsMobile(),
				IsDesktop:     d.IsDesktop(),
				IsTablet:      d.IsTablet(),
				IsTouchscreen: d.IsTouchscreen(),
				IsBot:         d.UserAgentIsBot(),
			}
		}
	}

	return Profile{
		Geo:    geoInfo,
		Device: deviceInfo,
		Meta: MetaInfo{
			ClientIP:  clientIP,
			UserAgent: userAgent,
			RequestID: requestID,
		},
	}
}

func stringOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func parseUserAgent(ua string) DeviceInfo {
	lower := strings.ToLower(ua)

	if strings.Contains(lower, "ipad") {
		return DeviceInfo{Type: "tablet", Brand: "Apple", Model: "iPad", IsMobile: false, IsDesktop: false, IsTablet: true, IsTouchscreen: true, IsBot: false}
	}
	if strings.Contains(lower, "iphone") {
		return DeviceInfo{Type: "smartphone", Brand: "Apple", Model: "iPhone", IsMobile: true, IsDesktop: false, IsTablet: false, IsTouchscreen: true, IsBot: false}
	}
	if strings.Contains(lower, "android") && strings.Contains(lower, "mobile") {
		return DeviceInfo{Type: "smartphone", Brand: "Android", Model: "Phone", IsMobile: true, IsDesktop: false, IsTablet: false, IsTouchscreen: true, IsBot: false}
	}
	if strings.Contains(lower, "android") {
		return DeviceInfo{Type: "tablet", Brand: "Android", Model: "Tablet", IsMobile: false, IsDesktop: false, IsTablet: true, IsTouchscreen: true, IsBot: false}
	}
	if strings.Contains(lower, "bot") || strings.Contains(lower, "crawl") || strings.Contains(lower, "spider") || strings.Contains(lower, "slurp") || strings.Contains(lower, "googlebot") {
		return DeviceInfo{Type: "bot", Brand: "N/A", Model: "Crawler", IsMobile: false, IsDesktop: false, IsTablet: false, IsTouchscreen: false, IsBot: true}
	}
	return DeviceInfo{Type: "desktop", Brand: "Unknown", Model: "Desktop", IsMobile: false, IsDesktop: true, IsTablet: false, IsTouchscreen: false, IsBot: false}
}
