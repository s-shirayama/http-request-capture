package main

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type Request struct {
	URL      string      `json:"url"`
	Method   string      `json:"method"`
	Header   http.Header `json:"header"`
	Form     url.Values  `json:"form"`
	PostForm url.Values  `json:"post_form"`
}

type Response struct {
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header"`
	BodyBase64 string      `json:"body_base64,omitempty"`
	Body       string      `json:"body,omitempty"`
	BodyData   string      `json:"-"`
}

type Call struct {
	Id       string   `json:"id"`
	Request  Request  `json:"request"`
	Response Response `json:"response"`
}

func CreateRequest(r *http.Request) Request {
	return Request{
		URL:      r.URL.String(),
		Method:   r.Method,
		Header:   r.Header,
		Form:     r.Form,
		PostForm: r.PostForm,
	}
}

func CreateResponse(r *http.Response) Response {
	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	bodyString := string(bodyBytes)
	delete(r.Header, "Content-Length")
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	return Response{
		StatusCode: r.StatusCode,
		Header:     r.Header,
		BodyBase64: base64.StdEncoding.EncodeToString(bodyBytes),
		Body:       bodyString,
		BodyData:   bodyString,
	}
}

func PrepareRequest(r *http.Request, scheme string) {
	// Need to be reset
	r.RequestURI = ""
	if r.URL.Scheme == "" {
		r.URL.Scheme = scheme
		r.URL.Host = r.Host
	}

	// Since PostForm() consume Request.Body so that it needs to be set again for client.Do()
	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	// This sets Request.PostForm
	r.ParseForm()
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
}

func SendProxyRequest(r *http.Request, proxyURLStr string) *http.Response {
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		log.Println(err)
	}
	transport := &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
	}
	resp, err := client.Do(r)
	if err != nil {
		log.Fatal(err)
	}
	return resp
}

var calls = make(map[string]Call)
var mappings = make(map[string]Call)

func MatchRequest(r *http.Request) *Response {
	for _, mapping := range mappings {
		if mapping.Request.URL == r.URL.String() &&
			mapping.Request.Method == r.Method {
			return &mapping.Response
		}
	}
	return nil
}

func gunzipWrite(w io.Writer, data []byte) error {
	// Write gzipped data to the client
	gr, err := gzip.NewReader(bytes.NewBuffer(data))
	defer gr.Close()
	data, err = ioutil.ReadAll(gr)
	if err != nil {
		return err
	}
	w.Write(data)
	return nil
}

func CreateTempFile(name string, contents []byte) string {
	tmpFile, err := ioutil.TempFile("tmp", name)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpFile.Write(contents); err != nil {
		log.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}
	return tmpFile.Name()
}

var (
	g errgroup.Group
)

func main() {
	g.Go(func() error {
		server := &http.Server{
			Addr:    ":8080",
			Handler: adminRouter(),
		}
		return server.ListenAndServe()
	})

	g.Go(func() error {
		proxy := goproxy.NewProxyHttpServer()
		proxy.NonproxyHandler = nonproxyHandler(proxy, "http")
		proxy.OnRequest().DoFunc(requestProxyHandler("http", func(r *http.Request) (*http.Request, *http.Response) { return r, nil }))
		proxy.OnResponse().DoFunc(onResponseProxyHandler)
		return http.ListenAndServe(":8081", proxy)
	})

	g.Go(func() error {
		proxy := goproxy.NewProxyHttpServer()
		proxy.NonproxyHandler = nonproxyHandler(proxy, "https")
		proxy.OnRequest().DoFunc(requestProxyHandler("https", func(r *http.Request) (*http.Request, *http.Response) { return r, nil }))
		proxy.OnResponse().DoFunc(onResponseProxyHandler)

		tmpCertFileName := CreateTempFile("temp.crt", goproxy.CA_CERT)
		tmpKeyFileName := CreateTempFile("temp.key", goproxy.CA_KEY)
		go func() {
			time.Sleep(1 * time.Second)
			os.Remove(tmpCertFileName)
			os.Remove(tmpKeyFileName)
		}()

		return http.ListenAndServeTLS(":8082", tmpCertFileName, tmpKeyFileName, proxy)
	})

	if os.Getenv("PROXY_URL") != "" && os.Getenv("PROXY_PORT") != "" {
		g.Go(func() error {
			proxy := goproxy.NewProxyHttpServer()
			proxy.OnRequest().DoFunc(requestProxyHandler("", func(req *http.Request) (*http.Request, *http.Response) {
				resp := SendProxyRequest(req, os.Getenv("PROXY_URL")+":"+os.Getenv("PROXY_PORT"))
				return req, resp
			}))
			proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
			proxy.OnResponse().DoFunc(onResponseProxyHandler)
			return http.ListenAndServe(":8083", proxy)
		})
	}

	if err := g.Wait(); err != nil {
		log.Fatal(err)
		log.Printf("ERROR!")
	}
}

func nonproxyHandler(proxy *goproxy.ProxyHttpServer, scheme string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host == "" {
			fmt.Fprintln(w, "Cannot handle requests without Host header, e.g., HTTP 1.0")
			return
		}
		req.URL.Scheme = scheme
		req.URL.Host = req.Host
		proxy.ServeHTTP(w, req)
	})
}
func requestProxyHandler(scheme string, defaultAction func(r *http.Request) (*http.Request, *http.Response)) func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		PrepareRequest(req, scheme)
		response := MatchRequest(req)
		if response != nil {
			resp := &http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString(response.BodyData)),
				Header:     response.Header,
				StatusCode: response.StatusCode,
			}
			return req, resp
		}
		return defaultAction(req)
	}
}

func onResponseProxyHandler(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp.Request != nil {
		request := CreateRequest(resp.Request)
		response := CreateResponse(resp)

		id := uuid.New().String()
		calls[id] = Call{
			Id:       id,
			Request:  request,
			Response: response,
		}
	}
	return resp
}

func adminRouter() http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())
	r.StaticFile("/", "./index.html")
	r.GET("/requests", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"count": len(calls),
			"data":  calls,
		})
	})
	r.DELETE("/requests", func(c *gin.Context) {
		calls = make(map[string]Call)
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	r.GET("/mappings", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"count": len(mappings),
			"data":  mappings,
		})
	})
	r.GET("/files/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.Header("Content-Type", mappings[id].Response.Header["Content-Type"][0])
		c.String(200, mappings[id].Response.BodyData)
	})
	r.PUT("/files/:id", func(c *gin.Context) {
		id := c.Param("id")
		defer c.Request.Body.Close()
		bodyBytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.Fatal(err)
		}
		mapping := mappings[id]
		mapping.Response.BodyData = string(bodyBytes)
		mappings[id] = mapping
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	r.POST("/recordings/:id", func(c *gin.Context) {
		id := c.Param("id")
		call := calls[id]
		if h, ok := call.Response.Header["Content-Encoding"]; ok && h[0] == "gzip" {
			var buf bytes.Buffer
			err := gunzipWrite(&buf, []byte(call.Response.Body))
			if err != nil {
				log.Fatal(err)
			}
			call.Response.BodyData = buf.String()
			delete(call.Response.Header, "Content-Encoding")
		}
		call.Response.Body = ""
		call.Response.BodyBase64 = ""
		mappings[id] = call
		c.JSON(200, gin.H{
			"data": calls[id],
		})
	})
	r.DELETE("/mappings/:id", func(c *gin.Context) {
		id := c.Param("id")
		delete(mappings, id)
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	r.GET("/mappings/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(200, gin.H{
			"data": mappings[id],
		})
	})
	r.PUT("/mappings/:id", func(c *gin.Context) {
		id := c.Param("id")
		var mapping Call
		c.BindJSON(&mapping)
		if id != mapping.Id {
			log.Fatal("Id is different")
		} else {
			mappings[id] = mapping
		}
		c.JSON(200, gin.H{
			"data": mapping,
		})
	})

	return r
}
