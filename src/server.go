package main

import (
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

type router struct {
	// For Custom Pattern URL
	handlers map[string]map[string]HandlerFunc
}

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type HandlerFunc func(*Context)

type Context struct {
	para  map[string]interface{}
	res_w http.ResponseWriter
	req   *http.Request
}

func loghandler(next HandlerFunc) HandlerFunc {
	return func(c *Context) {
		t := time.Now()
		next(c)

		log.Printf("[%s] %q %v\n",
			c.req.Method,
			c.req.URL.String(),
			time.Now().Sub(t))
	}
}

func recoverhandler(next HandlerFunc) HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic : %+v", err)
				http.Error(
					c.res_w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
		}()
		next(c)
	}
}

func staticHandler(next HandlerFunc) HandlerFunc {
	var (
		dir       = http.Dir(".")
		indexFile = "index.html"
	)
	return func(c *Context) {
		if c.req.Method != "GET" && c.req.Method != "HEAD" {
			next(c)
			return
		}

		file := c.req.URL.Path
		f, err := dir.Open(file)
		if err != nil {
			next(c)
			return
		}
		defer f.Close()

		ff, err := f.Stat()
		if err != nil {
			next(c)
			return
		}

		if ff.IsDir() {
			if !strings.HasSuffix(c.req.URL.Path, "/") {
				http.Redirect(c.res_w, c.req, c.req.URL.Path+"/", http.StatusFound)
				return
			}

			file = path.Join(file, indexFile)
			f, err = dir.Open(file)
			if err != nil {
				next(c)
				return
			}
			defer f.Close()

			ff, err := f.Stat()
			if err != nil || ff.IsDir() {
				next(c)
				return
			}
		}

		http.ServeContent(c.res_w, c.req, file, ff.ModTime(), f)
	}
}

func match(pattern, path string) (bool, map[string]string) {
	if pattern == path {
		return true, nil
	}

	patterns := strings.Split(pattern, "/")
	paths := strings.Split(path, "/")

	if len(patterns) != len(paths) {
		return false, nil
	}

	params := make(map[string]string)

	for i := 0; i < len(patterns); i++ {
		switch {
		case patterns[i] == paths[i]:
		case len(patterns[i]) > 0 && patterns[i][0] == ':':
			params[patterns[i][1:]] = paths[i]
		default:
			return false, nil
		}
	}
	return true, params
}

func (r *router) ServeHTTP(w http.ResponseWriter, re *http.Request) {
	for pattern, handler := range r.handlers[re.Method] {
		if ok, params := match(pattern, re.URL.Path); ok {
			c := Context{
				para:  make(map[string]interface{}),
				res_w: w,
				req:   re,
			}
			for method, data := range params {
				c.para[method] = data
			}

			handler(&c)
			return
		}
	}

	http.NotFound(w, re)
	return
}

func (r *router) HandleFunc(method, pattern string, h HandlerFunc) {
	m, ok := r.handlers[method] // method : GET, POST, PUT, DELETE
	if !ok {
		m = make(map[string]HandlerFunc)
		r.handlers[method] = m
	}
	m[pattern] = h
}

func SetPage(r *router, method, page, str string) {
	r.HandleFunc(method, page, func(c *Context) {
		fmt.Fprintln(c.res_w, str)
	})
}

func main() {
	r := &router{make(map[string]map[string]HandlerFunc)}

	SetPage(r, "GET", "/", "Welcome !")
	SetPage(r, "GET", "/about", "This is ABOUT Page !")

	r.HandleFunc("GET", "/public/index.html",
		loghandler(recoverhandler(staticHandler(func(c *Context) {}))))
	r.HandleFunc("GET", "/users/:id", loghandler(recoverhandler(func(c *Context) {
		if c.para["id"] == "0" {
			panic("id is zero")
		}
		fmt.Fprintf(c.res_w, "Retrieve User id : %v\n", c.para["id"])
	})))

	SetPage(r, "POST", "/users", "Create User Page !")

	r.HandleFunc("GET", "/users/:user_id/addr/:addr_id", loghandler(func(c *Context) {
		fmt.Fprintf(c.res_w, "Retrieve User id : %v, Address id : %v\n",
			c.para["user_id"],
			c.para["addr_id"])
	}))

	r.HandleFunc("POST", "/users/:user_id/addr/", loghandler(func(c *Context) {
		fmt.Fprintf(c.res_w, "Retrieve User id : %v\n", c.para["user_id"])
	}))
	http.ListenAndServe(":8080", r)
}
