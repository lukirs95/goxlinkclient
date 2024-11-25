package xlinkclient

import "fmt"

type Logger interface{
	Info(string)
	Infof(string, ...interface{})
	Warn(string)
	Warnf(string, ...interface{})
	Error(error)
	Errorf(string, ...interface{})
}

type DefaultLogger struct {}

func (l DefaultLogger) Info(val string) {
	fmt.Printf("INFO::%s\n", val)
}
func (l DefaultLogger) Infof(val string, opts ...interface{}) {
	l.Info(fmt.Sprintf(val, opts...))
}
func (l DefaultLogger) Warn(val string) {
	fmt.Printf("WARN::%s\n", val)
}
func (l DefaultLogger) Warnf(val string, opts ...interface{}) {
	l.Warn(fmt.Sprintf(val, opts...))
}
func (l DefaultLogger) Error(err error) {
	fmt.Printf("ERROR::%s", err.Error())
}
func (l DefaultLogger) Errorf(err string, opts ...interface{}) {
	l.Error(fmt.Errorf(err, opts...))
}