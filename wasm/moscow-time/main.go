package main

import (
	"encoding/json"
	"net/http"
	"time"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
)

type TimeResponse struct {
	Unix       int64  `json:"unix"`
	ISO        string `json:"iso"`
	HourMinute string `json:"hour_minute"`
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		utcNow := time.Now().UTC()
		now := utcNow.Add(3 * time.Hour)

		result := TimeResponse{
			Unix:       utcNow.Unix(),
			ISO:        now.Format("2006-01-02T15:04:05+03:00"),
			HourMinute: now.Format("15:04"),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})
}

func main() {}
