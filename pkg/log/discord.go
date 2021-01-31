package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	base "log"
	"net/http"
	"os"
	"time"
)

type DiscordLogger struct {
	url    string
	ch     chan string
	client *http.Client
	Logger Logger

	stop chan<- struct{}
	done <-chan struct{}
}

func NewDiscord(url string) Logger {
	l := &DiscordLogger{
		url:    url,
		ch:     make(chan string, 1000),
		client: &http.Client{Timeout: 3 * time.Second},
		Logger: &CoreLogger{
			Level:  4,
			Logger: base.New(os.Stderr, "", base.LstdFlags|base.Llongfile),
		},
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		l.poster(stop)
		close(done)
	}()
	l.stop = stop
	l.done = done
	return l
}

func (l *DiscordLogger) Fatal(v ...interface{}) {
	l.Logger.Fatal(v...)
}

func (l *DiscordLogger) Printf(format string, v ...interface{}) {
	l.Post(fmt.Sprintf(format, v...))
	l.Logger.Printf(format, v...)
}

func (l *DiscordLogger) Print(v ...interface{}) {
	l.Post(fmt.Sprintln(v...))
	l.Logger.Print(v...)
}

func (l *DiscordLogger) Debug(v ...interface{}) {
	l.Logger.Debug(v...)
}

func (l *DiscordLogger) Debugf(format string, v ...interface{}) {
	l.Logger.Debugf(format, v...)
}

func (l *DiscordLogger) Close() {
	close(l.stop)
	defer func() {
		l.Logger.Close()
		<-l.done
	}()
	for {
		select {
		case text := <-l.ch:
			if err := l.post(text); err != nil {
				l.Logger.Print(err)
			}
		default:
			return
		}
	}
}

func (l *DiscordLogger) Post(text string) {
	select {
	case l.ch <- text:
	default:
		l.Logger.Print("discord channel is full, skip", text)
	}
}

func (l *DiscordLogger) poster(stop <-chan struct{}) {
	for {
		select {
		case text := <-l.ch:
			if err := l.post(text); err != nil {
				l.Logger.Print(err)
			}
		case <-stop:
			return
		}

	}
}

func (l *DiscordLogger) post(text string) error {
	body, err := json.Marshal(map[string]string{"username": "camelia", "content": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, l.url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := l.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode/100 == 2 {
		return nil
	}
	return errors.New(res.Status)
}
