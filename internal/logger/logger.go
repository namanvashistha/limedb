package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

// OTELLogger wraps OTEL structured logging
type OTELLogger struct {
	otelLogger log.Logger
	slogLogger *slog.Logger
	nodeURL    string
}

// New creates a new OTEL logger instance
func New(nodeURL string) *OTELLogger {
	// Get OTEL logger from global provider
	otelLogger := global.GetLoggerProvider().Logger("limedb")
	
	// Also create slog for console output
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
	
	return &OTELLogger{
		otelLogger: otelLogger,
		slogLogger: slogLogger,
		nodeURL:    nodeURL,
	}
}

// logToOTEL sends structured log to OTEL
func (l *OTELLogger) logToOTEL(ctx context.Context, severity log.Severity, msg string, attrs ...log.KeyValue) {
	// Add standard attributes
	allAttrs := []log.KeyValue{
		log.String("node.url", l.nodeURL),
		log.String("service.name", "limedb"),
	}
	
	// Add trace context if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		spanCtx := span.SpanContext()
		allAttrs = append(allAttrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}
	
	// Add custom attributes
	allAttrs = append(allAttrs, attrs...)
	
	// Create log record
	record := log.Record{}
	record.SetBody(log.StringValue(msg))
	record.SetSeverity(severity)
	record.AddAttributes(allAttrs...)
	
	// Emit to OTEL
	l.otelLogger.Emit(ctx, record)
}

// Info logs an info message
func (l *OTELLogger) Info(ctx context.Context, msg string, attrs ...any) {
	// Console output
	l.slogLogger.InfoContext(ctx, msg, attrs...)
	
	// OTEL output
	otelAttrs := l.convertAttrs(attrs...)
	l.logToOTEL(ctx, log.SeverityInfo, msg, otelAttrs...)
}

// Error logs an error message  
func (l *OTELLogger) Error(ctx context.Context, msg string, attrs ...any) {
	// Console output
	l.slogLogger.ErrorContext(ctx, msg, attrs...)
	
	// OTEL output
	otelAttrs := l.convertAttrs(attrs...)
	l.logToOTEL(ctx, log.SeverityError, msg, otelAttrs...)
}

// Debug logs a debug message
func (l *OTELLogger) Debug(ctx context.Context, msg string, attrs ...any) {
	// Console output
	l.slogLogger.DebugContext(ctx, msg, attrs...)
	
	// OTEL output
	otelAttrs := l.convertAttrs(attrs...)
	l.logToOTEL(ctx, log.SeverityDebug, msg, otelAttrs...)
}

// Warn logs a warning message
func (l *OTELLogger) Warn(ctx context.Context, msg string, attrs ...any) {
	// Console output
	l.slogLogger.WarnContext(ctx, msg, attrs...)
	
	// OTEL output
	otelAttrs := l.convertAttrs(attrs...)
	l.logToOTEL(ctx, log.SeverityWarn, msg, otelAttrs...)
}

// convertAttrs converts slog-style attributes to OTEL log attributes
func (l *OTELLogger) convertAttrs(attrs ...any) []log.KeyValue {
	var otelAttrs []log.KeyValue
	
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			key := fmt.Sprintf("%v", attrs[i])
			value := fmt.Sprintf("%v", attrs[i+1])
			otelAttrs = append(otelAttrs, log.String(key, value))
		}
	}
	
	return otelAttrs
}

// Global logger instance
var DefaultLogger *OTELLogger

// Init initializes the global logger
func Init(nodeURL string) {
	DefaultLogger = New(nodeURL)
}

// Simple global functions - no context needed for most cases
func Info(msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Info(context.Background(), msg, attrs...)
	}
}

func Error(msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Error(context.Background(), msg, attrs...)
	}
}

func Debug(msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Debug(context.Background(), msg, attrs...)
	}
}

func Warn(msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Warn(context.Background(), msg, attrs...)
	}
}

// Context-aware functions (only when you have a request context with trace info)
func InfoCtx(ctx context.Context, msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Info(ctx, msg, attrs...)
	}
}

func ErrorCtx(ctx context.Context, msg string, attrs ...any) {
	if DefaultLogger != nil {
		DefaultLogger.Error(ctx, msg, attrs...)
	}
}