package main

import (
	"github.com/BurntSushi/toml"
	"github.com/Sirupsen/logrus"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type Renderer interface {
	Render(w io.Writer, page Page)
}

type TemplateRenderer struct {
	Dir string
	*template.Template
}

func (tr *TemplateRenderer) Render(w io.Writer, page Page) {
	if page.Template == "" {
		page.Template = "default"
	}
	if tr.Template == nil || tr.Lookup(page.Template) == nil {
		logger.WithField("nil-template", tr.Template == nil).Debug("Needs to load template")
		tr.Load(page.Template) //Not loaded yet!
	}
	tr.ExecuteTemplate(w, page.Template, page)
}

func (tr *TemplateRenderer) Load(name string) {
	if tr.Template == nil {
		tr.Template = template.New("default")
		tr.Load("default")
	}
	if name == "default" && tr.Dir == "" {
		logger.Info("Parsing default static template.")
		template.Must(tr.Parse(pageTemplate)) //Load hardcoded default instead.
		return
	}

	filename := filepath.Join(tr.Dir, name) + ".tmpl.html"
	logger.WithFields(logrus.Fields{
		"name":     name,
		"filename": filename,
	}).Info("Loading template")

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"err":      err,
			"template": name,
			"file":     filename,
		}).Error("Error encountered reading template contents.")
		return
	}

	var t *template.Template
	if name != "default" {
		t = tr.New(name)
	} else {
		t = tr.Template
	}
	template.Must(t.Parse(string(data)))

}

type Pages interface {
	Get(name string) Page
}

const (
	MainMenuPage            = "menu"
	LoginPage               = "login"
	LoginSuccessPage        = "login_success"
	RegistrationPage        = "register"
	RegistrationSuccessPage = "register_success"
	LogoutPage              = "logout"
	AdminPage               = "admin"
	Error404Page            = "404"
)

type Page struct {
	Template string
	Title    template.HTML
	Head     template.HTML
	Content  template.HTML
}

type FSPages struct {
	webdir string
	cache  map[string]*pageCacheData
}

type pageCacheData struct {
	Page
	NotFound     bool
	Filename     string
	LastLoadTime time.Time
}

func NewFSPages(dir string) *FSPages {
	return &FSPages{
		webdir: dir,
		cache:  make(map[string]*pageCacheData),
	}
}

func (fsp FSPages) Get(name string) Page {
	if data, ok := fsp.cache[name]; ok {
		//File is in cache, Check if update is necessary
		finfo, err := os.Stat(data.Filename)
		if err != nil || finfo.ModTime().After(data.LastLoadTime) {
			if os.IsNotExist(err) {
				if name == Error404Page {
					return Page{ //Default to prevent infinite loops if the error page can not be loaded.
						Title:   "404 - Page Not Found",
						Content: "<p>Could not find neither the requested page not the proper 404 page.</p>",
					}
				}
				return fsp.Get(Error404Page)
			} else {
				fsp.Load(name)       //Modification time after last load
				return fsp.Get(name) //Recursive call. Should be loaded now, but might not been found.
			}
		}
		return data.Page
	} else {
		//Not loaded before. Do it.
		fsp.Load(name)
		return fsp.Get(name) //Recursive call since it is now in the cache.
	}
}

func (fsp FSPages) Load(name string) {
	pcd := pageCacheData{
		Filename:     filepath.Join(fsp.webdir, "pages", name) + ".toml",
		LastLoadTime: time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"name":     name,
		"filename": pcd.Filename,
	}).Info("Loading page")

	_, err := toml.DecodeFile(pcd.Filename, &pcd.Page)
	if os.IsNotExist(err) {
		pcd.NotFound = true
	}
	fsp.cache[name] = &pcd
}

type ConstPages map[string]Page

var constPages = make(ConstPages)

func (cp ConstPages) Get(name string) Page {
	if page, ok := cp[name]; ok {
		return page
	} else {
		return Page{
			Title: "<code>/dev/nil</code>",
			Head:  "",
			Content: `
	<p>
		You appear to have tried to access a page that does not exist.
	</p>`,
		}
	}
}

const pageTemplate = `<html>
<head>
	<title>AuthProx - {{.Title}}</title>
	<link rel="stylesheet" href="/proxy/static/master.css" media="screen" charset="utf-8">
	<link href='http://fonts.googleapis.com/css?family=Roboto+Condensed' rel='stylesheet' type='text/css'>
	{{.Head}}
</head>

<body>
	<div id="card">
		<h3>AuthProx</h3>
		<h1>{{.Title}}</h1>
		{{.Content}}
	</div>
</body>
</html>
`

func init() {
	constPages[LoginPage] = Page{
		Title: "Login",
		Content: `
	<form method="POST" action="/proxy/login">
		<input type="text" name="username" placeholder="Username" required>
		<input type="password" name="password" placeholder="Password" required>
		<input type="submit" value="Login">
	</form>`,
	}

	constPages[LoginSuccessPage] = Page{
		Title: "Login Success!",
		Content: `
	<p>
		You have successfully logged in.
		<a href="/">Continue</a>
	</p>`,
	}

	constPages[RegistrationPage] = Page{
		Title: "Registrations",
		Head:  "<script src='https://www.google.com/recaptcha/api.js'></script>",
		Content: `
	<form method="POST" action="/proxy/register">
		<input type="text" name="username" placeholder="Username" required>
		<input type="password" name="password" placeholder="Password" required>
		<div class="g-recaptcha" data-sitekey="6LcMDgoTAAAAALJTFmdzPieTUheKAdghSG9q1_D-"></div>
		<input type="submit" value="Register">
	</form>`,
	}

	constPages[RegistrationSuccessPage] = Page{
		Title: "Registration Successfull!",
		Content: `
	<p>
		You have successfully registered your new account.
		<a href="/proxy/login">Continue to login page</a>
	</p>`,
	}

	constPages[LogoutPage] = Page{
		Title: "Logout Successfull",
		Content: `
		<p>
			You have successfully been logged out.
		</p>
		<p>
			Please return at a later time!
		</p>
		`,
	}
}
