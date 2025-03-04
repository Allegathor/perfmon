package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	defcfg "github.com/Allegathor/perfmon/internal"
	"github.com/Allegathor/perfmon/internal/storage"
	"github.com/go-chi/chi/v5"
)

func CreateRootHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		htmlStr := `
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Metrics</title>
			<style>
				table {
					border-collapse: collapse;
					border: 2px solid rgb(140 140 140);
					font-family: sans-serif;
					font-size: 0.8rem;
					letter-spacing: 1px;
				}

				caption {
					caption-side: bottom;
					padding: 10px;
					font-weight: bold;
				}

				thead,
				tfoot {
					background-color: rgb(228 240 245);
				}

				th,
				td {
					border: 1px solid rgb(160 160 160);
					padding: 8px 10px;
				}

				td:last-of-type {
					text-align: center;
				}

				tbody > tr:nth-of-type(even) {
					background-color: rgb(237 238 242);
				}

				tfoot th {
					text-align: right;
				}

				tfoot td {
					font-weight: bold;
				}
			</style>
		</head>
		<body>
		`
		htmlStr += fmt.Sprintf("<table>\n<caption>%s</caption>\n", defcfg.TypeCounter)
		htmlStr += `
			<thead>
				<tr>
					<th scope="col">Name</th>
					<th scope="col">Value</th>
				</tr>
			</thead>
			<tbody>
		`
		for k, v := range s.Counter {
			tr := `
				<tr>
					<th scope="row">%s</th>
					<td>%d</td>
				</tr>
			`
			htmlStr += fmt.Sprintf(tr, k, v)
		}
		htmlStr += `
			</tbody>
		</table>
		`

		htmlStr += fmt.Sprintf("<table>\n<caption>%s</caption>\n", defcfg.TypeGauge)
		htmlStr += `
			<thead>
				<tr>
					<th scope="col">Name</th>
					<th scope="col">Value</th>
				</tr>
			</thead>
			<tbody>
		`
		for k, v := range s.Gauge {
			tr := `
				<tr>
					<th scope="row">%s</th>
					<td>%f</td>
				</tr>
			`
			htmlStr += fmt.Sprintf(tr, k, v)
		}
		htmlStr += `
			</tbody>
		</table>
		</body>
		`
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := rw.Write([]byte(htmlStr))
		if err != nil {
			http.Error(rw, "rw error", http.StatusInternalServerError)
		}
	}
}

func CreateUpdateHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		t := chi.URLParam(req, defcfg.URLTypePath)
		n := chi.URLParam(req, defcfg.URLNamePath)
		v := chi.URLParam(req, defcfg.URLValuePath)
		// fmt.Printf("%s, %s, %s", t, n, v)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == defcfg.TypeGauge {
			gv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
			}
			s.SetGauge(n, gv)
			rw.WriteHeader(http.StatusOK)

		} else if t == defcfg.TypeCounter {
			cv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				http.Error(rw, "invalid value", http.StatusBadRequest)
			}
			s.SetCounter(n, cv)
			rw.WriteHeader(http.StatusOK)

		} else {
			http.Error(rw, "incorrect request type", http.StatusBadRequest)
		}

	}
}

func CreateValueHandler(s *storage.MetricsStorage) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		t := req.PathValue(defcfg.URLTypePath)
		n := req.PathValue(defcfg.URLNamePath)

		if n == "" {
			http.Error(rw, "name must contain a value", http.StatusNotFound)
			return
		}

		if t == defcfg.TypeGauge {

			if v, ok := s.Gauge[n]; ok {
				_, err := rw.Write([]byte(strconv.FormatFloat(v, 'f', 3, 64)))
				if err != nil {
					http.Error(rw, "rw error", http.StatusInternalServerError)
				}
				return
			}

			http.Error(rw, "value doesn't exist in storage", http.StatusNotFound)

		} else if t == defcfg.TypeCounter {
			if v, ok := s.Counter[n]; ok {
				_, err := rw.Write([]byte(strconv.FormatInt(v, 10)))
				if err != nil {
					http.Error(rw, "rw error", http.StatusInternalServerError)
				}
				return
			}

			http.Error(rw, "value doesn't exist in storage", http.StatusNotFound)

		} else {
			http.Error(rw, "incorrect request type", http.StatusBadRequest)
		}

	}
}
