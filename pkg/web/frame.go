package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jolestar/go-probe/pkg/probe"
	"github.com/yunify/metad/atomic"
	yaml "gopkg.in/yaml.v2"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"github.com/jolestar/go-probe/pkg/httputil"
)

const (
	ContentHtml     = 1
	ContentTypeHtml = "text/html"
	ContentJSON     = 2
	ContentTypeJSON = "application/json"
	ContentYAML     = 3
	ContentTypeYAML = "application/yaml"
)

type Config struct {
	Listen string `yaml:"listen"`
}

type Frame struct {
	router       *mux.Router
	config       *Config
	requestIDGen atomic.AtomicLong
}

func New(config *Config) (*Frame, error) {
	return &Frame{router: mux.NewRouter(), config: config}, nil
}

func (f *Frame) Init() {
	f.initRouter()
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

func (f *Frame) initRouter() {
	f.router.HandleFunc("/favicon.ico", http.NotFound)

	f.router.HandleFunc("/", f.handleWrapper(f.root)).Methods("GET")
	f.router.HandleFunc("/{probeName:.*}", f.handleWrapper(f.root)).Methods("GET")
}

func (f *Frame) root(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	probeName := vars["probeName"]
	ctx = context.WithValue(ctx, "request", req)
	r, err := probe.DoProbe(ctx, probeName)
	if err != nil {
		httpErr, ok := err.(*HttpError)
		if ok {
			return nil, httpErr
		} else {
			return nil, NewServerError(err)
		}
	}
	return r, nil
}

type handleFunc func(ctx context.Context, req *http.Request) (interface{}, *HttpError)

func (f *Frame) handleWrapper(handler handleFunc) func(w http.ResponseWriter, req *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		requestID := f.generateRequestID()

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
			f.errorLog(requestID, status, err.Message)
		} else {
			if result == nil {
				respondSuccessDefault(w, req)
			} else {
				len = respondSuccess(w, req, result)
			}
		}
		f.requestLog(requestID, req, status, elapsed, len)
	}
}

func (f *Frame) requestIP(req *http.Request) string {
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

func (f *Frame) generateRequestID() string {
	return fmt.Sprintf("REQ-%d", f.requestIDGen.IncrementAndGet())
}

func (f *Frame) Serve() {
	log.Printf("Listening on %s \n", f.config.Listen)
	log.Fatal("%v", http.ListenAndServe(f.config.Listen, f.router))
}

type RequestLog struct {
	RequestID string
	RequestMethod string
	RequestIP string
	RequestURI string
	RequestContentLength int64
	ResponseStatus int
	ResponseTime int64
	ResponseSize int
}

func (f *Frame) requestLog(requestID string, req *http.Request, status int, elapsed time.Duration, len int) {
	reqLog := RequestLog{
		RequestID: requestID,
		RequestMethod: req.Method,
		RequestIP: f.requestIP(req),
		RequestURI: req.URL.RequestURI(),
		RequestContentLength: req.ContentLength,
		ResponseStatus: status,
		ResponseTime: int64(elapsed/time.Millisecond),
		ResponseSize: len,
	}
	b, err := json.Marshal(reqLog)
	if err != nil {
		log.Printf("Error to marshal reqLog %+v\n", reqLog)
	}else {
		log.Printf(string(b))
	}
}

func (f *Frame) errorLog(requestID string, status int, msg string) {
	log.Printf("ERR %s %v %s\n", requestID, status, msg)
}

func contentType(req *http.Request) int {
	str := httputil.NegotiateContentType(req, []string{
		"text/plain",
		"text/html",
		"application/json",
		"application/yaml",
		"application/x-yaml",
		"text/x-yaml",
	}, "text/plain")

	if strings.Contains(str, "json") {
		return ContentJSON
	} else if strings.Contains(str, "yaml") {
		return ContentYAML
	} else {
		return ContentHtml
	}
}

func respondError(w http.ResponseWriter, req *http.Request, msg string, statusCode int) {
	obj := make(map[string]interface{})
	obj["message"] = msg
	obj["type"] = "ERROR"
	obj["code"] = statusCode

	switch contentType(req) {
	case ContentHtml:
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
	case ContentHtml:
		respondHtml(w, req, "OK")
	case ContentJSON:
		respondJSON(w, req, obj)
	case ContentYAML:
		respondYAML(w, req, obj)
	}
}

func respondSuccess(w http.ResponseWriter, req *http.Request, val interface{}) int {
	switch contentType(req) {
	case ContentHtml:
		return respondHtml(w, req, val)
	case ContentJSON:
		return respondJSON(w, req, val)
	case ContentYAML:
		respondYAML(w, req, val)
	}
	return 0
}

var (
	listTemplate   *template.Template
	resultTemplate *template.Template
	initErr        error
)

func init() {
	listTemplate, initErr = template.New("listTemplate").Parse(`{{range .}}<a href="{{.Name}}">{{.Name}}</a><br/>{{end}}`)
	if initErr != nil {
		panic(initErr)
	}
	resultTemplate, initErr = template.New("resultTemplate").Parse(`<h2>{{.Name}}</h2><h4>{{.Summary}}</h4><table>{{range $k,$v := .Data}}<tr><td>{{$k}}</td><td>{{$v}}</td></tr>{{end}}</table>`)
	if initErr != nil {
		panic(initErr)
	}
}

func respondHtml(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeHtml)
	if val == nil {
		fmt.Fprint(w, "")
		return 0
	}
	var buffer bytes.Buffer
	var err error
	switch val.(type) {
	case []*probe.Result:
		err = listTemplate.Execute(&buffer, val)
	case *probe.Result:
		err = resultTemplate.Execute(&buffer, val)
	default:
		log.Fatalf("Value is of a type I don't know how to handle: %+v \n", val)
	}
	if err != nil {
		buffer.Reset()
		buffer.WriteString(fmt.Sprintf("response error: %s", err.Error()))
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

func respondYAML(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeYAML)
	bytes, err := yaml.Marshal(val)
	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to YAML: "+err.Error(), http.StatusInternalServerError)
	}
	return len(bytes)
}
