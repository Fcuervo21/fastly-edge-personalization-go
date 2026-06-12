package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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
		case "/":
			profile := buildProfile(r)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Edge-Personalization", "true")
			w.WriteHeader(fsthttp.StatusOK)
			fmt.Fprint(w, renderPage(profile))
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

type Greeting struct {
	Text string
	Flag string
	Lang string
}

func getLocalizedGreeting(countryCode string) Greeting {
	greetings := map[string]Greeting{
		"US": {Text: "Welcome", Flag: "\U0001F1FA\U0001F1F8", Lang: "English"},
		"GB": {Text: "Welcome", Flag: "\U0001F1EC\U0001F1E7", Lang: "English"},
		"JP": {Text: "こんにちは", Flag: "\U0001F1EF\U0001F1F5", Lang: "日本語"},
		"DE": {Text: "Willkommen", Flag: "\U0001F1E9\U0001F1EA", Lang: "Deutsch"},
		"FR": {Text: "Bienvenue", Flag: "\U0001F1EB\U0001F1F7", Lang: "Français"},
		"ES": {Text: "Bienvenido", Flag: "\U0001F1EA\U0001F1F8", Lang: "Español"},
		"BR": {Text: "Bem-vindo", Flag: "\U0001F1E7\U0001F1F7", Lang: "Português"},
		"IN": {Text: "नमस्ते", Flag: "\U0001F1EE\U0001F1F3", Lang: "Hindi"},
		"AU": {Text: "G'day", Flag: "\U0001F1E6\U0001F1FA", Lang: "English"},
		"KR": {Text: "안녕하세요", Flag: "\U0001F1F0\U0001F1F7", Lang: "Korean"},
	}
	if g, ok := greetings[countryCode]; ok {
		return g
	}
	return Greeting{Text: "Hello", Flag: "\U0001F30D", Lang: "Default"}
}

type ContentCard struct {
	Title string
	Desc  string
	Icon  string
}

type Recommendations struct {
	Theme string
	Cards []ContentCard
}

func getContentRecommendations(continent string) Recommendations {
	recommendations := map[string]Recommendations{
		"NA": {
			Theme: "Tech & Innovation",
			Cards: []ContentCard{
				{Title: "Edge Computing Guide", Desc: "How sub-millisecond latency transforms real-time APIs and WebSocket connections across North American POPs.", Icon: "⚡"},
				{Title: "Compute@Edge Performance", Desc: "Benchmarking WASM execution at Fastly POPs vs traditional CDN edge workers.", Icon: "\U0001F680"},
				{Title: "API Gateway Patterns", Desc: "Route, transform, and authorize API traffic at the edge before it touches your origin.", Icon: "\U0001F50C"},
			},
		},
		"EU": {
			Theme: "Privacy & Compliance",
			Cards: []ContentCard{
				{Title: "GDPR at the Edge", Desc: "Strip PII, enforce consent headers, and geo-fence data without origin round-trips.", Icon: "\U0001F6E1️"},
				{Title: "Data Sovereignty", Desc: "Keep request processing within EU POPs using Fastly's geolocation-aware routing.", Icon: "\U0001F3DB️"},
				{Title: "Privacy-First Analytics", Desc: "Aggregate visitor signals at the edge — no cookies, no client-side tracking scripts.", Icon: "\U0001F4CA"},
			},
		},
		"AS": {
			Theme: "Mobile-First Optimization",
			Cards: []ContentCard{
				{Title: "Mobile Edge Delivery", Desc: "Optimize payloads for mobile networks across APAC — adaptive image sizing, Brotli compression at the POP.", Icon: "\U0001F4F1"},
				{Title: "Low-Latency Streaming", Desc: "HLS/DASH manifest manipulation at the edge for Asia-Pacific audiences.", Icon: "\U0001F3AC"},
				{Title: "Progressive Web Apps", Desc: "Service worker pre-caching strategies powered by Fastly Compute edge logic.", Icon: "\U0001F310"},
			},
		},
	}
	if r, ok := recommendations[continent]; ok {
		return r
	}
	return Recommendations{
		Theme: "Global Edge Network",
		Cards: []ContentCard{
			{Title: "Edge Compute Overview", Desc: "Run custom logic at 90+ Fastly POPs worldwide with zero cold starts.", Icon: "\U0001F30D"},
			{Title: "Instant Purge", Desc: "Invalidate cached content globally in ~150ms with Fastly's purge API.", Icon: "\U0001F9F9"},
			{Title: "Origin Shield", Desc: "Reduce origin load by collapsing requests through a shield POP.", Icon: "\U0001F6E1️"},
		},
	}
}

func renderPage(profile Profile) string {
	greeting := getLocalizedGreeting(profile.Geo.CountryCode)
	recs := getContentRecommendations(profile.Geo.Continent)

	deviceType := "desktop"
	if profile.Device.IsMobile {
		deviceType = "mobile"
	} else if profile.Device.IsTablet {
		deviceType = "tablet"
	}

	gridCols := map[string]string{"mobile": "1fr", "tablet": "1fr 1fr", "desktop": "1fr 1fr 1fr"}
	columns := gridCols[deviceType]

	var contentCards strings.Builder
	for _, c := range recs.Cards {
		fmt.Fprintf(&contentCards, `
    <div class="card content-card">
      <div class="card-icon">%s</div>
      <h4>%s</h4>
      <p>%s</p>
    </div>`, c.Icon, html.EscapeString(c.Title), html.EscapeString(c.Desc))
	}

	lat := "N/A"
	lng := "N/A"
	if profile.Geo.Latitude != nil {
		lat = fmt.Sprintf("%.4f", *profile.Geo.Latitude)
	}
	if profile.Geo.Longitude != nil {
		lng = fmt.Sprintf("%.4f", *profile.Geo.Longitude)
	}

	utcOffsetDisplay := "?"
	if profile.Geo.UTCOffset != nil {
		offset := *profile.Geo.UTCOffset
		if offset >= 0 {
			utcOffsetDisplay = fmt.Sprintf("+%d", offset)
		} else {
			utcOffsetDisplay = fmt.Sprintf("%d", offset)
		}
	}

	utcOffsetInspect := "N/A"
	if profile.Geo.UTCOffset != nil {
		utcOffsetInspect = fmt.Sprintf("%d", *profile.Geo.UTCOffset)
	}

	boolClass := func(b bool) string {
		if b {
			return "bool-true"
		}
		return "bool-false"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-device="%s">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Edge Personalization Engine — Fastly Compute Demo</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    background: #0f0f13;
    color: #e2e2e8;
    min-height: 100vh;
    line-height: 1.6;
  }

  /* --- Header --- */
  header {
    text-align: center;
    padding: 3rem 1.5rem 2.5rem;
    background: linear-gradient(170deg, #1a1a24 0%%, #0f0f13 100%%);
    border-bottom: 1px solid #2a2a36;
  }
  .logo-badge {
    display: inline-block;
    font-size: .7rem;
    font-weight: 700;
    letter-spacing: .12em;
    text-transform: uppercase;
    color: #ff282d;
    border: 1px solid #ff282d;
    border-radius: 4px;
    padding: .25rem .7rem;
    margin-bottom: 1rem;
  }
  header h1 {
    font-size: 2rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ff282d 0%%, #ff6b6b 100%%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    margin-bottom: .5rem;
  }
  header .subtitle {
    color: #9090a0;
    font-size: .95rem;
    max-width: 640px;
    margin: 0 auto;
  }

  /* --- Greeting Banner --- */
  .greeting {
    text-align: center;
    padding: 2rem 1.5rem;
    background: #12121a;
  }
  .greeting .flag { font-size: 2.5rem; margin-bottom: .5rem; }
  .greeting h2 { font-size: 1.6rem; color: #fff; margin-bottom: .25rem; }
  .greeting .location {
    font-size: .85rem;
    color: #8888a0;
  }
  .greeting .location strong { color: #ff6b6b; }

  /* --- Main container --- */
  main {
    max-width: 960px;
    margin: 0 auto;
    padding: 2rem 1.5rem 3rem;
  }

  /* --- Section headers --- */
  .section-label {
    font-size: .75rem;
    font-weight: 700;
    letter-spacing: .1em;
    text-transform: uppercase;
    color: #ff282d;
    margin-bottom: .75rem;
  }
  .section-title {
    font-size: 1.2rem;
    color: #c0c0d0;
    margin-bottom: 1.25rem;
  }

  /* --- Content cards grid --- */
  .content-grid {
    display: grid;
    grid-template-columns: %s;
    gap: 1rem;
    margin-bottom: 3rem;
  }

  /* --- Cards --- */
  .card {
    background: #16161e;
    border: 1px solid #2a2a36;
    border-radius: 12px;
    padding: 1.25rem;
    transition: border-color .15s, transform .15s;
  }
  .card:hover { border-color: #ff282d; transform: translateY(-2px); }
  .content-card .card-icon { font-size: 1.5rem; margin-bottom: .6rem; }
  .content-card h4 { font-size: .95rem; color: #fff; margin-bottom: .4rem; }
  .content-card p { font-size: .8rem; color: #8888a0; line-height: 1.55; }

  /* --- Edge Inspection Dashboard --- */
  .dashboard {
    background: #111119;
    border: 1px solid #2a2a36;
    border-radius: 14px;
    padding: 2rem;
    margin-bottom: 2rem;
  }
  .dashboard-header {
    display: flex;
    align-items: center;
    gap: .75rem;
    margin-bottom: 1.5rem;
  }
  .dashboard-header .pulse {
    width: 10px; height: 10px;
    background: #ff282d;
    border-radius: 50%%;
    animation: pulse 2s ease-in-out infinite;
  }
  @keyframes pulse {
    0%%, 100%% { opacity: 1; box-shadow: 0 0 0 0 rgba(255,40,45,.4); }
    50%% { opacity: .7; box-shadow: 0 0 0 6px rgba(255,40,45,0); }
  }
  .dashboard-header h3 {
    font-size: 1rem;
    color: #fff;
    font-weight: 700;
  }
  .dashboard-header .badge {
    font-size: .6rem;
    font-weight: 700;
    letter-spacing: .08em;
    text-transform: uppercase;
    background: #1b2e1b;
    color: #4ade80;
    padding: .2rem .6rem;
    border-radius: 20px;
    margin-left: auto;
  }

  /* --- Inspection panels --- */
  .inspect-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  @media (max-width: 600px) {
    .inspect-grid { grid-template-columns: 1fr; }
  }
  .inspect-panel {
    background: #16161e;
    border: 1px solid #22222e;
    border-radius: 10px;
    padding: 1.1rem;
  }
  .inspect-panel h4 {
    font-size: .75rem;
    font-weight: 700;
    letter-spacing: .08em;
    text-transform: uppercase;
    color: #ff6b6b;
    margin-bottom: .8rem;
    display: flex;
    align-items: center;
    gap: .4rem;
  }
  .inspect-row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: .3rem 0;
    border-bottom: 1px solid #1e1e28;
    font-size: .8rem;
  }
  .inspect-row:last-child { border-bottom: none; }
  .inspect-key {
    color: #7070a0;
    font-family: "SF Mono", "Fira Code", "Consolas", monospace;
    font-size: .75rem;
  }
  .inspect-val {
    color: #e2e2e8;
    font-family: "SF Mono", "Fira Code", "Consolas", monospace;
    font-size: .75rem;
    text-align: right;
    max-width: 60%%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .bool-true { color: #4ade80; }
  .bool-false { color: #666; }

  /* --- Flow diagram --- */
  .flow {
    display: flex;
    align-items: center;
    gap: .4rem;
    justify-content: center;
    flex-wrap: wrap;
    margin-bottom: 1rem;
    padding: 1rem;
    background: #16161e;
    border-radius: 10px;
    border: 1px solid #22222e;
  }
  .flow-step {
    background: #1e1e2a;
    border: 1px solid #2a2a36;
    border-radius: 6px;
    padding: .4rem .7rem;
    font-size: .7rem;
    color: #c0c0d0;
    font-family: "SF Mono", "Fira Code", "Consolas", monospace;
    white-space: nowrap;
  }
  .flow-step.active {
    border-color: #ff282d;
    color: #ff6b6b;
    background: #1e1418;
  }
  .flow-arrow { color: #4ade80; font-size: .9rem; }
  .flow-label {
    width: 100%%;
    text-align: center;
    font-size: .7rem;
    color: #555;
    margin-top: .3rem;
  }

  /* --- Zero-origin badge --- */
  .zero-origin {
    text-align: center;
    padding: .8rem;
    background: linear-gradient(135deg, #1b1420 0%%, #14141e 100%%);
    border: 1px solid #2a2236;
    border-radius: 8px;
    font-size: .75rem;
    color: #b090c0;
  }
  .zero-origin strong { color: #ff6b6b; }

  /* --- Footer --- */
  footer {
    text-align: center;
    padding: 2rem 1rem;
    font-size: .75rem;
    color: #444;
    border-top: 1px solid #1e1e28;
  }
  footer a { color: #ff6b6b; text-decoration: none; }
  footer code {
    background: #1a1a24;
    padding: .15rem .4rem;
    border-radius: 4px;
    font-size: .7rem;
    color: #666;
  }

  /* --- Built with go badge --- */
  .lang-badge {
    position: fixed;
    bottom: 1.5rem;
    left: 1.5rem;
    background: rgba(0, 173, 216, 0.15);
    color: #00ADD8;
    border: 1px solid #00ADD8;
    border-radius: 6px;
    padding: 0.4rem 0.9rem;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    z-index: 9999;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, monospace;
  }
</style>
</head>
<body>

<header>
  <div class="logo-badge">Fastly Compute</div>
  <h1>Edge Personalization Engine</h1>
  <p class="subtitle">
    Dynamic edge content optimization — this page was compiled at a Fastly POP using
    geolocation headers and device detection. Zero origin. Zero latency.
  </p>
</header>

<div class="greeting">
  <div class="flag">%s</div>
  <h2>%s</h2>
  <p class="location">
    Personalized for <strong>%s, %s</strong>
    &middot; %s &middot; UTC%s
  </p>
</div>

<main>

  <!-- Content Recommendations -->
  <p class="section-label">%s</p>
  <p class="section-title">Content tailored to your region via Fastly Compute geolocation</p>
  <div class="content-grid">
    %s
  </div>

  <!-- Edge Inspection Dashboard -->
  <div class="dashboard">
    <div class="dashboard-header">
      <div class="pulse"></div>
      <h3>Edge Inspection Dashboard</h3>
      <span class="badge">Live</span>
    </div>

    <div class="inspect-grid">
      <div class="inspect-panel">
        <h4>🌍 Geolocation Headers</h4>
        <div class="inspect-row"><span class="inspect-key">country_code</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">country_name</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">city</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">region</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">continent</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">latitude</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">longitude</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">postal_code</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">utc_offset</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">as_name</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">conn_speed</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">conn_type</span><span class="inspect-val">%s</span></div>
      </div>

      <div class="inspect-panel">
        <h4>📱 Device Detection</h4>
        <div class="inspect-row"><span class="inspect-key">hardware_type</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">brand</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">model</span><span class="inspect-val">%s</span></div>
        <div class="inspect-row"><span class="inspect-key">is_mobile</span><span class="inspect-val %s">%t</span></div>
        <div class="inspect-row"><span class="inspect-key">is_desktop</span><span class="inspect-val %s">%t</span></div>
        <div class="inspect-row"><span class="inspect-key">is_tablet</span><span class="inspect-val %s">%t</span></div>
        <div class="inspect-row"><span class="inspect-key">is_touchscreen</span><span class="inspect-val %s">%t</span></div>
        <div class="inspect-row"><span class="inspect-key">is_bot</span><span class="inspect-val %s">%t</span></div>
        <div class="inspect-row" style="margin-top:.5rem;border-top:1px solid #2a2a36;padding-top:.5rem">
          <span class="inspect-key">client_ip</span><span class="inspect-val">%s</span>
        </div>
        <div class="inspect-row"><span class="inspect-key">layout_mode</span><span class="inspect-val" style="color:#ff6b6b">%s</span></div>
      </div>
    </div>

    <!-- Micro-flow diagram -->
    <div class="flow">
      <span class="flow-step">Client Request</span>
      <span class="flow-arrow">→</span>
      <span class="flow-step active">Fastly POP</span>
      <span class="flow-arrow">→</span>
      <span class="flow-step">Geo Lookup</span>
      <span class="flow-arrow">→</span>
      <span class="flow-step">Device Detect</span>
      <span class="flow-arrow">→</span>
      <span class="flow-step active">HTML Compiled</span>
      <span class="flow-arrow">→</span>
      <span class="flow-step">Response</span>
      <p class="flow-label">Entire request lifecycle executes at the edge — no origin server involved</p>
    </div>

    <div class="zero-origin">
      This HTML was <strong>compiled natively at the Fastly edge</strong> using Compute services.
      Geolocation and device data were resolved at the POP in sub-millisecond time —
      <strong>zero origin latency</strong>, zero round-trips, zero cold starts.
    </div>
  </div>

</main>

<footer>
  Powered by <a href="https://www.fastly.com/products/compute" target="_blank" rel="noopener">Fastly Compute</a>
  &middot; Request <code>%s</code>
</footer>

<div class="lang-badge">Built with go</div>

</body>
</html>`,
		html.EscapeString(deviceType),
		columns,
		greeting.Flag,
		html.EscapeString(greeting.Text),
		html.EscapeString(profile.Geo.City),
		html.EscapeString(profile.Geo.CountryName),
		html.EscapeString(greeting.Lang),
		html.EscapeString(utcOffsetDisplay),
		html.EscapeString(recs.Theme),
		contentCards.String(),
		html.EscapeString(profile.Geo.CountryCode),
		html.EscapeString(profile.Geo.CountryName),
		html.EscapeString(profile.Geo.City),
		html.EscapeString(profile.Geo.Region),
		html.EscapeString(profile.Geo.Continent),
		html.EscapeString(lat),
		html.EscapeString(lng),
		html.EscapeString(profile.Geo.PostalCode),
		html.EscapeString(utcOffsetInspect),
		html.EscapeString(profile.Geo.AsName),
		html.EscapeString(profile.Geo.ConnSpeed),
		html.EscapeString(profile.Geo.ConnType),
		html.EscapeString(profile.Device.Type),
		html.EscapeString(profile.Device.Brand),
		html.EscapeString(profile.Device.Model),
		boolClass(profile.Device.IsMobile), profile.Device.IsMobile,
		boolClass(profile.Device.IsDesktop), profile.Device.IsDesktop,
		boolClass(profile.Device.IsTablet), profile.Device.IsTablet,
		boolClass(profile.Device.IsTouchscreen), profile.Device.IsTouchscreen,
		boolClass(profile.Device.IsBot), profile.Device.IsBot,
		html.EscapeString(profile.Meta.ClientIP),
		html.EscapeString(deviceType),
		html.EscapeString(profile.Meta.RequestID),
	)
}
