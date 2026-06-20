package service_test
import (
	"recipe-server/internal/service"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"

	"github.com/alicebob/miniredis/v2"
)

func TestWeatherFetchAndCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"current": {
				"temperature_2m": 28.5,
				"relative_humidity_2m": 65,
				"weather_code": 0
			}
		}`))
	}))
	t.Cleanup(srv.Close)

	config.AppConfig = &config.Config{
		Weather: config.WeatherConfig{
			Enabled:       true,
			Provider:      "open_meteo",
			DefaultCity:   "成都",
			DefaultLat:    30.57,
			DefaultLon:    104.06,
			CacheTTLHours: 3,
		},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	ws := service.NewWeatherService(store, &http.Client{Timeout: 5 * time.Second})
	service.SetWeatherAPIBaseForTest(ws, srv.URL)
	ctx := context.Background()
	w1, err := ws.GetDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if w1.City != "成都" || w1.TempC != 28.5 {
		t.Fatalf("w1: %+v", w1)
	}
	srv.Close()
	w2, err := ws.GetDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if w2.TempC != 28.5 {
		t.Fatalf("cache miss: %+v", w2)
	}
}

func TestWeatherCodeText(t *testing.T) {
	if service.WeatherCodeTextForTest(0) != "晴" {
		t.Fatal(service.WeatherCodeTextForTest(0))
	}
	if service.WeatherCodeTextForTest(61) != "雨" {
		t.Fatal(service.WeatherCodeTextForTest(61))
	}
}
