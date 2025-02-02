package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/qingcloudhx/contrib/trigger/rest/cors"
	"flogo/core/data/metadata"
	"flogo/core/support/log"
	"flogo/core/trigger"
)

const (
	CorsPrefix = "REST_TRIGGER"
)

var triggerMd = trigger.NewMetadata(&Settings{}, &HandlerSettings{}, &Output{}, &Reply{})

func init() {
	_ = trigger.Register(&Trigger{}, &Factory{})
}

type Factory struct {
}

// Metadata implements trigger.Factory.Metadata
func (*Factory) Metadata() *trigger.Metadata {
	return triggerMd
}

// New implements trigger.Factory.New
func (*Factory) New(config *trigger.Config) (trigger.Trigger, error) {
	s := &Settings{}
	err := metadata.MapToStruct(config.Settings, s, true)
	if err != nil {
		return nil, err
	}

	return &Trigger{id: config.Id, settings: s}, nil
}

// Trigger REST trigger struct
type Trigger struct {
	server   *Server
	settings *Settings
	id       string
	logger   log.Logger
}

func (t *Trigger) Initialize(ctx trigger.InitContext) error {

	t.logger = ctx.Logger()

	router := httprouter.New()

	addr := ":" + strconv.Itoa(t.settings.Port)

	pathMap := make(map[string]string)

	preflightHandler := &PreflightHandler{logger: t.logger, c: cors.New(CorsPrefix, t.logger)}

	// Init handlers
	for _, handler := range ctx.GetHandlers() {

		s := &HandlerSettings{}
		err := metadata.MapToStruct(handler.Settings(), s, true)
		if err != nil {
			return err
		}

		method := s.Method
		path := s.Path

		t.logger.Debugf("Registering handler [%s: %s]", method, path)

		if _, ok := pathMap[path]; !ok {
			pathMap[path] = path
			router.OPTIONS(path, preflightHandler.handleCorsPreflight) // for CORS
		}

		//router.OPTIONS(path, handleCorsPreflight) // for CORS
		router.Handle(method, path, newActionHandler(t, strings.ToUpper(method), handler))
	}

	t.logger.Debugf("Configured on port %d", t.settings.Port)

	var options []func(*Server)

	if t.settings.EnableTLS {
		options = append(options, TLS(t.settings.CertFile, t.settings.KeyFile))
	}

	server, err := NewServer(addr, router, options...)
	if err != nil {
		return err
	}

	t.server = server

	return nil
}

func (t *Trigger) Start() error {
	return t.server.Start()
}

// Stop implements util.Managed.Stop
func (t *Trigger) Stop() error {
	return t.server.Stop()
}

type PreflightHandler struct {
	logger log.Logger
	c      cors.Cors
}

// Handles the cors preflight request
func (h *PreflightHandler) handleCorsPreflight(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	h.logger.Debugf("Received [OPTIONS] request to CorsPreFlight: %+v", r)
	h.c.HandlePreflight(w, r)
}

// IDResponse id response object
type IDResponse struct {
	ID string `json:"id"`
}

func newActionHandler(rt *Trigger, method string, handler trigger.Handler) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		rt.logger.Debugf("Received request for id '%s'", rt.id)

		c := cors.New(CorsPrefix, rt.logger)
		c.WriteCorsActualRequestHeaders(w)

		out := &Output{}
		out.Method = method

		out.PathParams = make(map[string]string)
		for _, param := range ps {
			out.PathParams[param.Key] = param.Value
		}

		queryValues := r.URL.Query()
		out.QueryParams = make(map[string]string, len(queryValues))
		out.Headers = make(map[string]string, len(r.Header))

		for key, value := range r.Header {
			out.Headers[key] = strings.Join(value, ",")
		}

		for key, value := range queryValues {
			out.QueryParams[key] = strings.Join(value, ",")
		}

		// Check the HTTP Header Content-Type
		contentType := r.Header.Get("Content-Type")
		switch contentType {
		case "application/x-www-form-urlencoded":
			buf := new(bytes.Buffer)
			_,err :=buf.ReadFrom(r.Body)
			if err != nil {
				rt.logger.Debugf("Error reading body: %s", err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			s := buf.String()
			m, err := url.ParseQuery(s)
			if err != nil {
				rt.logger.Debugf("Error parsing query string: %s", err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			content := make(map[string]interface{}, 0)
			for key, val := range m {
				if len(val) == 1 {
					content[key] = val[0]
				} else {
					content[key] = val[0]
				}
			}

			out.Content = content
		case "application/json":
			var content interface{}
			err := json.NewDecoder(r.Body).Decode(&content)
			if err != nil {
				switch {
				case err == io.EOF:
					// empty body
					//todo what should handler say if content is expected?
				default:
					rt.logger.Debugf("Error parsing json body: %s", err.Error())
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			out.Content = content
		default:
			if strings.Contains(contentType, "multipart/form-data") {
				// need to still extract the body, only handling the multipart data for now...

				if err := r.ParseMultipartForm(32); err != nil {
					rt.logger.Debugf("Error parsing multipart form: %s", err.Error())
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				var files []map[string]interface{}

				for key, fh := range r.MultipartForm.File {
					for _, header := range fh {

						fileDetails, err := getFileDetails(key, header)
						if err != nil {
							rt.logger.Debugf("Error getting attached file details: %s", err.Error())
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}

						files = append(files, fileDetails)
					}
				}

				// The content output from the trigger
				content := map[string]interface{}{
					"body":  nil,
					"files": files,
				}
				out.Content = content
			} else {
				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					rt.logger.Debugf("Error reading body: %s", err.Error())
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				out.Content = string(b)
			}
		}

		results, err := handler.Handle(context.Background(), out)
		if err != nil {
			rt.logger.Debugf("Error handling request: %s", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		reply := &Reply{}
		err = reply.FromMap(results)
		if err != nil {
			rt.logger.Debugf("Error mapping results: %s", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if reply.Data != nil {

			if reply.Code == 0 {
				reply.Code = 200
			}

			switch t := reply.Data.(type) {
			case string:
				var v interface{}
				err := json.Unmarshal([]byte(t), &v)
				if err != nil {
					//Not a json
					w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
				} else {
					//Json
					w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				}

				w.WriteHeader(reply.Code)
				_, err = w.Write([]byte(t))
				if err != nil {
					rt.logger.Debugf("Error writing body: %s", err.Error())
				}
				return
			default:
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(reply.Code)
				if err := json.NewEncoder(w).Encode(reply.Data); err != nil {
					rt.logger.Debugf("Error encoding json reply: %s", err.Error())
				}
				return
			}
		}

		if reply.Code > 0 {
			w.WriteHeader(reply.Code)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}


func getFileDetails(key string, header *multipart.FileHeader) (map[string]interface{}, error){
	file, err := header.Open()
	if err != nil {
		return nil, err
	}

	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return nil, err
	}

	fileDetails := map[string]interface{}{
		"key":      key,
		"fileName": header.Filename,
		"fileType": header.Header.Get("Content-Type"),
		"size":     header.Size,
		"file":     buf.Bytes(),
	}

	return fileDetails, nil
}