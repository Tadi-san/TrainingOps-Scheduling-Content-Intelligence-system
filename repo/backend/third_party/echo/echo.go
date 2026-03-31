package echo

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type HandlerFunc func(Context) error
type MiddlewareFunc func(HandlerFunc) HandlerFunc

type Echo struct {
	HideBanner bool
	routes     []*route
	middleware []MiddlewareFunc
	cors       corsConfig
}

type corsConfig struct {
	enabled     bool
	origins     map[string]struct{}
	headers     string
	methods     string
	credentials bool
}

type route struct {
	method  string
	pattern string
	handler HandlerFunc
}

type Context interface {
	Request() *http.Request
	SetRequest(*http.Request)
	QueryParam(string) string
	Param(string) string
	Bind(any) error
	JSON(int, any) error
	ResponseWriter() http.ResponseWriter
	SetResponseWriter(http.ResponseWriter)
}

type context struct {
	req    *http.Request
	writer http.ResponseWriter
	params map[string]string
}

func New() *Echo {
	return &Echo{
		routes:     []*route{},
		middleware: []MiddlewareFunc{},
		cors: corsConfig{
			headers:     "Content-Type, Authorization, X-Requested-With, X-Tenant-ID",
			methods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
			credentials: true,
		},
	}
}

func (e *Echo) Use(m ...MiddlewareFunc) {
	e.middleware = append(e.middleware, m...)
}

func (e *Echo) GET(path string, handler HandlerFunc) {
	e.routes = append(e.routes, &route{method: http.MethodGet, pattern: path, handler: e.apply(handler)})
}

func (e *Echo) POST(path string, handler HandlerFunc) {
	e.routes = append(e.routes, &route{method: http.MethodPost, pattern: path, handler: e.apply(handler)})
}

func (e *Echo) PUT(path string, handler HandlerFunc) {
	e.routes = append(e.routes, &route{method: http.MethodPut, pattern: path, handler: e.apply(handler)})
}

func (e *Echo) DELETE(path string, handler HandlerFunc) {
	e.routes = append(e.routes, &route{method: http.MethodDelete, pattern: path, handler: e.apply(handler)})
}

func (e *Echo) EnableCORS(origins []string) {
	e.cors.enabled = len(origins) > 0
	e.cors.origins = map[string]struct{}{}
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		e.cors.origins[origin] = struct{}{}
	}
}

func (e *Echo) apply(handler HandlerFunc) HandlerFunc {
	for i := len(e.middleware) - 1; i >= 0; i-- {
		handler = e.middleware[i](handler)
	}
	return handler
}

func (e *Echo) Start(addr string) error {
	return http.ListenAndServe(addr, e)
}

func (e *Echo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e.handleCORS(w, r) {
		return
	}

	for _, rt := range e.routes {
		if r.Method != rt.method {
			continue
		}
		if params, ok := match(rt.pattern, r.URL.Path); ok {
			ctx := &context{req: r, writer: w, params: params}
			if err := rt.handler(ctx); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	http.NotFound(w, r)
}

func (e *Echo) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	if !e.cors.enabled {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		return false
	}

	if _, ok := e.cors.origins[origin]; !ok {
		return false
	}

	headers := w.Header()
	headers.Set("Vary", "Origin")
	headers.Set("Access-Control-Allow-Origin", origin)
	headers.Set("Access-Control-Allow-Methods", e.cors.methods)
	headers.Set("Access-Control-Allow-Headers", e.cors.headers)
	headers.Set("Access-Control-Allow-Credentials", "true")
	headers.Set("Access-Control-Max-Age", "600")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}

	return false
}

func (c *context) Request() *http.Request {
	return c.req
}

func (c *context) SetRequest(req *http.Request) {
	c.req = req
}

func (c *context) QueryParam(key string) string {
	return c.req.URL.Query().Get(key)
}

func (c *context) Param(name string) string {
	return c.params[name]
}

func (c *context) Bind(target any) error {
	defer c.req.Body.Close()
	return json.NewDecoder(c.req.Body).Decode(target)
}

func (c *context) JSON(status int, body any) error {
	c.writer.Header().Set("Content-Type", "application/json")
	c.writer.WriteHeader(status)
	return json.NewEncoder(c.writer).Encode(body)
}

func (c *context) ResponseWriter() http.ResponseWriter {
	return c.writer
}

func (c *context) SetResponseWriter(writer http.ResponseWriter) {
	c.writer = writer
}

func match(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return nil, false
	}
	params := map[string]string{}
	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], ":") {
			params[patternParts[i][1:]] = pathParts[i]
			continue
		}
		if patternParts[i] != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

func (c *context) String(status int, s string) error {
	c.writer.WriteHeader(status)
	_, err := io.WriteString(c.writer, s)
	return err
}
