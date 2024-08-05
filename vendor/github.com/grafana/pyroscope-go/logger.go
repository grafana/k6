package pyroscope

import "fmt"

// these loggers implement the types.Logger interface

type noopLoggerImpl struct{}

func (*noopLoggerImpl) Infof(_ string, _ ...interface{})  {}
func (*noopLoggerImpl) Debugf(_ string, _ ...interface{}) {}
func (*noopLoggerImpl) Errorf(_ string, _ ...interface{}) {}

type standardLoggerImpl struct{}

func (*standardLoggerImpl) Infof(a string, b ...interface{})  { fmt.Printf("[INFO]  "+a+"\n", b...) }
func (*standardLoggerImpl) Debugf(a string, b ...interface{}) { fmt.Printf("[DEBUG] "+a+"\n", b...) }
func (*standardLoggerImpl) Errorf(a string, b ...interface{}) { fmt.Printf("[ERROR] "+a+"\n", b...) }

var (
	noopLogger     = &noopLoggerImpl{}
	StandardLogger = &standardLoggerImpl{}
)
