package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"
)

// WeatherSnapshot 天气快照。
type WeatherSnapshot struct {
	City        string    `json:"city"`
	TempC       float64   `json:"temp_c"`
	WeatherText string    `json:"weather_text"`
	Humidity    int       `json:"humidity"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// HTTPDoer HTTP 客户端接口。
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// WeatherService 天气服务（Open-Meteo + Redis 缓存）。
type WeatherService struct {
	store   cache.Store
	client  HTTPDoer
	apiBase string
}

func NewWeatherService(store cache.Store, client HTTPDoer) *WeatherService {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &WeatherService{
		store:   store,
		client:  client,
		apiBase: "https://api.open-meteo.com/v1/forecast",
	}
}

func weatherCacheKey(city string) string {
	return fmt.Sprintf("weather:city:%s", city)
}

func (w *WeatherService) cfg() config.WeatherConfig {
	if config.AppConfig == nil {
		return config.WeatherConfig{
			Enabled: true, DefaultCity: "成都",
			DefaultLat: 30.5728, DefaultLon: 104.0668, CacheTTLHours: 3,
		}
	}
	return config.AppConfig.Weather
}

// GetDefault 获取默认城市天气（带缓存）。
func (w *WeatherService) GetDefault(ctx context.Context) (*WeatherSnapshot, error) {
	c := w.cfg()
	return w.GetByCoords(ctx, c.DefaultCity, c.DefaultLat, c.DefaultLon)
}

// GetByCoords 按坐标获取天气。
func (w *WeatherService) GetByCoords(ctx context.Context, city string, lat, lon float64) (*WeatherSnapshot, error) {
	c := w.cfg()
	if !c.Enabled {
		return &WeatherSnapshot{City: city, WeatherText: "未知", FetchedAt: time.Now()}, nil
	}
	key := weatherCacheKey(city)
	var cached WeatherSnapshot
	if err := w.store.GetJSON(ctx, key, &cached); err == nil {
		return &cached, nil
	}

	snap, err := w.fetchOpenMeteo(lat, lon, city)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(c.CacheTTLHours) * time.Hour
	_ = w.store.SetJSON(ctx, key, snap, ttl)
	return snap, nil
}

// SummaryForPrompt 供 AI Prompt 使用的单行摘要。
func (w *WeatherService) SummaryForPrompt(ctx context.Context) string {
	snap, err := w.GetDefault(ctx)
	if err != nil || snap == nil {
		return "天气信息暂不可用"
	}
	return fmt.Sprintf("%s 气温%.0f°C %s 湿度%d%%", snap.City, snap.TempC, snap.WeatherText, snap.Humidity)
}

type openMeteoResponse struct {
	Current struct {
		Temperature2m      float64 `json:"temperature_2m"`
		RelativeHumidity2m int     `json:"relative_humidity_2m"`
		WeatherCode        int     `json:"weather_code"`
	} `json:"current"`
}

func (w *WeatherService) fetchOpenMeteo(lat, lon float64, city string) (*WeatherSnapshot, error) {
	u, _ := url.Parse(w.apiBase)
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("current", "temperature_2m,relative_humidity_2m,weather_code")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("天气请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("天气 API 状态 %d", resp.StatusCode)
	}
	var om openMeteoResponse
	if err := json.Unmarshal(body, &om); err != nil {
		return nil, err
	}
	return &WeatherSnapshot{
		City:        city,
		TempC:       om.Current.Temperature2m,
		WeatherText: weatherCodeText(om.Current.WeatherCode),
		Humidity:    om.Current.RelativeHumidity2m,
		FetchedAt:   time.Now(),
	}, nil
}

func weatherCodeText(code int) string {
	switch {
	case code == 0:
		return "晴"
	case code >= 1 && code <= 3:
		return "多云"
	case code >= 45 && code <= 48:
		return "雾"
	case code >= 51 && code <= 67:
		return "雨"
	case code >= 71 && code <= 77:
		return "雪"
	case code >= 80 && code <= 82:
		return "阵雨"
	case code >= 95:
		return "雷雨"
	default:
		return "阴"
	}
}
