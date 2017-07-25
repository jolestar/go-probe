package main

import (
	"net/http"
	"github.com/gorilla/mux"
	"log"
	"fmt"
	"time"
	"context"
	"github.com/yunify/metad/atomic"
	"net"
	"bytes"
	"github.com/yunify/metad/util/flatmap"
	"sort"
	"encoding/json"
	"strings"
	"github.com/golang/gddo/httputil"
)

type Config struct {
	Listen   string   `yaml:"listen"`
}

type Probe struct {
	router       *mux.Router
	config       *Config
	requestIDGen atomic.AtomicLong
}

func New(config *Config) (*Probe, error) {
	return &Probe{router:mux.NewRouter(), config: config}, nil
}

func (p *Probe) Init() {
	p.initRouter()
}

type HttpError struct {
	Status  int
	Message string
}

func NewHttpError(status int, Message string) *HttpError {
	return &HttpError{Status: status, Message: Message}
}

func NewServerError(error error) *HttpError {
	return &HttpError{Status: http.StatusInternalServerError, Message: error.Error()}
}

func (e HttpError) Error() string {
	return fmt.Sprintf("%s", e.Message)
}

func (p *Probe) initRouter() {
	p.router.HandleFunc("/favicon.ico", http.NotFound)

	p.router.HandleFunc("/", p.handleWrapper(p.root)).
		Methods("GET", "HEAD")
}

func (p *Probe) root(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	return "HelloWorld", nil
}

type handleFunc func(ctx context.Context, req *http.Request) (interface{}, *HttpError)

func (m *Probe) handleWrapper(handler handleFunc) func(w http.ResponseWriter, req *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		requestID := m.generateRequestID()

		ctx := context.WithValue(req.Context(), "requestID", requestID)
		cancelCtx, cancelFun := context.WithCancel(ctx)
		if x, ok := w.(http.CloseNotifier); ok {
			closeNotify := x.CloseNotify()
			go func() {
				select {
				case <-closeNotify:
					cancelFun()
				}
			}()
		} else {
			defer cancelFun()
		}
		result, err := handler(cancelCtx, req)

		w.Header().Add("X-RequestID", requestID)
		elapsed := time.Since(start)
		status := 200
		var len int
		if err != nil {
			status = err.Status
			respondError(w, req, err.Message, status)
			m.errorLog(requestID, req, status, err.Message)
		} else {
			if result == nil {
				respondSuccessDefault(w, req)
			} else {
				len = respondSuccess(w, req, result)
			}
		}
		m.requestLog(requestID, req, status, elapsed, len)
	}
}

func (m *Probe) requestIP(req *http.Request) string {
	clientIp := req.Header.Get("X-Forwarded-For")
	if len(clientIp) > 0 {
		return clientIp
	}
	clientIp, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		log.Fatalf("Get RequestIP error: %s\n", err.Error())
	}
	return clientIp
}

func (m *Probe) generateRequestID() string {
	return fmt.Sprintf("REQ-%d", m.requestIDGen.IncrementAndGet())
}

func (p *Probe) Serve() {
	log.Printf("Listening on %s \n", p.config.Listen)
	log.Fatal("%v", http.ListenAndServe(p.config.Listen, p.router))
}

func (m *Probe) requestLog(requestID string, req *http.Request, status int, elapsed time.Duration, len int) {
	log.Printf("%s\t%s\t%s\t%s\t%v\t%v\t%v\t%v\n", requestID, req.Method, m.requestIP(req), req.URL.RequestURI(), req.ContentLength, status, int64(elapsed.Seconds()*1000), len)
}

func (m *Probe) errorLog(requestID string, req *http.Request, status int, msg string) {
	if status == 500 {
		log.Fatalf("ERR\t%s\t%s\t%s\t%s\t%v\t%v\t%s\n", requestID, req.Method, m.requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	} else {
		log.Printf("ERR\t%s\t%s\t%s\t%s\t%v\t%v\t%s\n", requestID, req.Method, m.requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	}
}

const (
	ContentText     = 1
	ContentTypeText = "text/plain"
	ContentJSON     = 2
	ContentTypeJSON = "application/json"
)

func contentType(req *http.Request) int {
	str := httputil.NegotiateContentType(req, []string{
		"text/plain",
		"application/json",
		"application/yaml",
		"application/x-yaml",
		"text/yaml",
		"text/x-yaml",
	}, "text/plain")

	if strings.Contains(str, "json") {
		return ContentJSON
	} else {
		return ContentText
	}
}

func respondError(w http.ResponseWriter, req *http.Request, msg string, statusCode int) {
	obj := make(map[string]interface{})
	obj["message"] = msg
	obj["type"] = "ERROR"
	obj["code"] = statusCode

	switch contentType(req) {
	case ContentText:
		http.Error(w, msg, statusCode)
	case ContentJSON:
		bytes, err := json.Marshal(obj)
		if err == nil {
			http.Error(w, string(bytes), statusCode)
		} else {
			http.Error(w, "{\"type\": \"error\", \"message\": \"JSON marshal error\"}", http.StatusInternalServerError)
		}
	}
}

func respondSuccessDefault(w http.ResponseWriter, req *http.Request) {
	obj := make(map[string]interface{})
	obj["type"] = "OK"
	obj["code"] = 200
	switch contentType(req) {
	case ContentText:
		respondText(w, req, "OK")
	case ContentJSON:
		respondJSON(w, req, obj)
	}
}

func respondSuccess(w http.ResponseWriter, req *http.Request, val interface{}) int {
	switch contentType(req) {
	case ContentText:
		return respondText(w, req, val)
	case ContentJSON:
		return respondJSON(w, req, val)
	}
	return 0
}

func respondText(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeText)
	if val == nil {
		fmt.Fprint(w, "")
		return 0
	}
	var buffer bytes.Buffer
	switch v := val.(type) {
	case string:
		buffer.WriteString(v)
	case map[string]interface{}:
		fm := flatmap.Flatten(v)
		var keys []string
		for k := range fm {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			buffer.WriteString(k)
			buffer.WriteString("\t")
			buffer.WriteString(fm[k])
			buffer.WriteString("\n")
		}
	default:
		log.Fatalf("Value is of a type I don't know how to handle: %v \n", val)
	}
	w.Write(buffer.Bytes())
	return buffer.Len()
}

func respondJSON(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeJSON)
	if val == nil {
		val = make(map[string]string)
	}
	prettyParam := req.FormValue("pretty")
	pretty := prettyParam != "" && prettyParam != "false"
	var bytes []byte
	var err error
	if pretty {
		bytes, err = json.MarshalIndent(val, "", "  ")
	} else {
		bytes, err = json.Marshal(val)
	}

	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to JSON: "+err.Error(), http.StatusInternalServerError)
	}
	return len(bytes)
}