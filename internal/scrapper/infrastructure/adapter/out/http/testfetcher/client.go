package testfetcher

import (
	"encoding/json"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	mu      sync.Mutex
	updates map[string]application.FetchResult
}

func NewClient() *Client {
	f := &Client{
		updates: make(map[string]application.FetchResult),
	}

	go f.startDirtyServer()

	return f
}

func (f *Client) CanHandle(url string) bool {
	return true
}

func (f *Client) Fetch(url string) (application.FetchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if result, ok := f.updates[url]; ok {
		delete(f.updates, url)
		return result, nil
	}

	return application.FetchResult{
		UpdatedAt:   time.Time{},
		Description: "nothing",
	}, nil
}

func (f *Client) startDirtyServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL         string `json:"url"`
			Description string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		f.mu.Lock()
		f.updates[req.URL] = application.FetchResult{
			UpdatedAt:   time.Now(),
			Description: req.Description,
		}
		f.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Update injected successfully!"))
	})

	err := http.ListenAndServe(":9999", mux)
	if err != nil {
		panic("failed to start dirty test server: " + err.Error())
	}
}
