package util

import (
"log"
"net/http"
"os"

"cloud.google.com/go/profiler"
"contrib.go.opencensus.io/exporter/stackdriver"
"contrib.go.opencensus.io/integrations/ocsql"
"go.opencensus.io/plugin/ochttp"
"go.opencensus.io/trace"
)

func InitProfiler(service string) {
	if err := profiler.Start(profiler.Config{
		Service:        service,
		ServiceVersion: "1.0.1",
		ProjectID:      os.Getenv("GOOGLE_CLOUD_PROJECT"),
	}); err != nil {
		log.Fatal(err)
	}
}

func InitTrace() {
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:                os.Getenv("GOOGLE_CLOUD_PROJECT"),
		TraceSpansBufferMaxBytes: 32 * 1024 * 1024,
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)

	trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(0.05)})
}

func WithTrace(h http.Handler) http.Handler {
	return &ochttp.Handler{Handler: h}
}

func TracedDriver(driverName string) string {
	driverName, err := ocsql.Register(driverName, ocsql.WithQuery(true), ocsql.WithQueryParams(true))
	if err != nil {
		log.Fatal(err)
	}
	return driverName
}
