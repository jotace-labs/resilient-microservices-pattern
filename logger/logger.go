package log

import (
	"context"
	"os"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// type ctxKey string
const traceIDKey string = "trace_id"  

func InitLogger(development bool) *zap.Logger {
	var encoderConfig zapcore.EncoderConfig
	var encoder zapcore.Encoder
	var level zapcore.LevelEnabler
	
	if development  {
		encoderConfig = zap.NewDevelopmentEncoderConfig() // plain text encoder
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // time format
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // display different colors for different levels
		// encoderConfig.EncodeCaller = zapcore.FullCallerEncoder // display full file path (/home/username/.../file.go)
	
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
		level = zapcore.DebugLevel

	} else {
		encoderConfig = zap.NewProductionEncoderConfig() // JSON encoder 
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // level is capital: INFO, DEBUG, ERROR
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // time format // similar to rfc 3339

		encoder = zapcore.NewJSONEncoder(encoderConfig)
		level = zapcore.InfoLevel
	}
	

	core := zapcore.NewCore(
		encoder, 
		zapcore.AddSync(os.Stdout), 
		level,
	)

	logger = zap.New(
		core, 
		zap.AddCaller(), 
		// zap.AddCallerSkip(1), // when using wrappers around zap, this prevents the shown path to be the wrapper
		) // this addCaller also display the path to the file, but relative


	return logger
}

func InitResources(ctx context.Context) context.Context {
	traceId := uuid.NewString()
	ctx = context.WithValue(ctx, "trace_id", traceId)

	return ctx
}

// L gets a context and tries to put a field of trace_id from this context into the returned logger
func L(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if id, ok := ctx.Value(traceIDKey).(string); ok {
			return logger.With(zap.String("trace_id", id))
		}
	} 

	return logger
}
