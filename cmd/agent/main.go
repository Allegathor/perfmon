package main

import (
	"net/http"

	collector "github.com/Allegathor/perfmon/internal/collector"
	"github.com/Allegathor/perfmon/internal/monclient"
)

func main() {
	client := monclient.NewInstance("http://localhost", 8080, 10)
	col := collector.New(2)
	ch := make(chan collector.Mtcs)
	go col.Monitor(ch)
	for mtcs := range ch {
		client.PollStats(mtcs.Gauge, mtcs.Counter)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	http.ListenAndServe(":8080", nil)
}
