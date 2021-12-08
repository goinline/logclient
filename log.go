package logclient

import (
	"bytes"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/TingYunGo/goagent/libs/pool"
)

type Service struct {
	used int32
}

func (s *Service) Stop() {
	for s.used == 0 {
		time.Sleep(1 * time.Microsecond)
	}
	atomic.AddInt32(&s.used, 1)
	for s.used <= 2 {
		time.Sleep(1 * time.Millisecond)
	}
}
func (s *Service) Start(worker func(running func() bool)) {
	s.used = 0
	go func() {
		inRunning := func() bool {
			return s.used == 1
		}
		atomic.AddInt32(&s.used, 1)
		worker(inRunning)
		atomic.AddInt32(&s.used, 1)
	}()
}

type message struct {
	data string
}
type logClass struct {
	messagePool pool.SerialReadPool
	svc         Service
	levelString string
}

func (l *logClass) append_data(data string) {
	if l == nil {
		return
	}
	l.messagePool.Put(&message{data})
}

func (l *logClass) init() *logClass {
	l.messagePool.Init()
	l.svc.Start(l.loop)
	return l
}

var logPostURL string = ""

func (l *logClass) processMessage() int {
	messageWrited := 0
	var buffer bytes.Buffer
	for l.messagePool.Size() > 0 {
		for msg := l.messagePool.Get(); msg != nil; msg = l.messagePool.Get() {
			if len(logPostURL) > 0 {
				buffer.WriteString(msg.(*message).data)
			}
			messageWrited++
		}
	}
	if messageWrited > 0 && len(logPostURL) > 0 {
		PostHttpRequest(logPostURL, map[string]string{}, buffer.Bytes(), time.Second*3)
	}
	return messageWrited
}

func (l *logClass) loop(running func() bool) {

	sleepDuration := time.Millisecond
	lastWrited := 1
	for running() {
		messageWrited := l.processMessage()
		if messageWrited == 0 {
			if lastWrited == 0 && sleepDuration < 100*time.Millisecond {
				sleepDuration *= 2
			}
			time.Sleep(sleepDuration)
		} else {
			sleepDuration = time.Millisecond
		}
		lastWrited = messageWrited
	}
	l.processMessage()
}

type logWrapper struct {
	loginst     *logClass
	pid         int
	levelString string
}

func (l *logWrapper) Printf(format string, a ...interface{}) {
	l.Append(fmt.Sprintf("%s (pid:%d) %s :", time.Now().Format("2006-01-02 15:04:05.000"), l.pid, l.levelString) + fmt.Sprintf(format, a...))
}

func (l *logWrapper) Print(a ...interface{}) {
	l.Append(fmt.Sprintf("%s (pid:%d) %s :", time.Now().Format("2006-01-02 15:04:05.000"), l.pid, l.levelString) + fmt.Sprint(a...))
}

func (l *logWrapper) Println(a ...interface{}) {
	l.Append(fmt.Sprintf("%s (pid:%d) %s :", time.Now().Format("2006-01-02 15:04:05.000"), l.pid, l.levelString) + fmt.Sprintln(a...))
}
func (l *logWrapper) init(level string, inst *logClass) {
	l.loginst = inst
	l.pid = os.Getpid()
	l.levelString = level
}
func (l *logWrapper) Append(data string) {
	if l == nil {
		return
	}
	if l.loginst == nil {
		return
	}
	l.loginst.append_data(data)
}

var inst logClass
var Error logWrapper
var Info logWrapper
var Warning logWrapper

func init() {
	logPostURL = os.Getenv("HTTP_LOG_URL")
	inst.init()
	Error.init("ERROR", &inst)
	Info.init("INFO", &inst)
	Warning.init("WARNING", &inst)
}
