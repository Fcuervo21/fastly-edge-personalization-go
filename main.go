package main

import (
	"context"
	"fmt"

	"github.com/fastly/compute-sdk-go/fsthttp"
)

func main() {
	fsthttp.ServeFunc(func(ctx context.Context, w fsthttp.ResponseWriter, r *fsthttp.Request) {
		switch r.URL.Path {
		case "/_edge/healthcheck":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(fsthttp.StatusOK)
			fmt.Fprint(w, "ok")
		default:
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(fsthttp.StatusNotFound)
			fmt.Fprint(w, "Not Found\n")
		}
	})
}
