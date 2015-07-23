package main

import (
	"html/template"
	"io"
)

type Renderer interface {
	Render(w io.Writer, page Page)
}

type TemplateRenderer struct {
	*template.Template
}

var defaultRenderer = TemplateRenderer{
	Template: template.New("authprox"),
}

func init() {
	template.Must(defaultRenderer.Parse(pageTemplate))
}

func (tr TemplateRenderer) Render(w io.Writer, page Page) {
	tr.Execute(w, page)
}

type Pages interface {
	Get(name string) Page
}

const (
	MainMenuPage            = "menu"
	LoginPage               = "login"
	LoginSuccessPage        = "login/success"
	RegistrationPage        = "register"
	RegistrationSuccessPage = "register/success"
	LogoutPage              = "logout"
	AdminPage               = "admin"
)

type Page struct {
	Title   template.HTML
	Head    template.HTML
	Content template.HTML
}

type ConstPages map[string]Page

var constPages = make(ConstPages)

func (cp ConstPages) Get(name string) Page {
	if page, ok := cp[name]; ok {
		return page
	} else {
		return Page{
			Title:   "<code>/dev/nil</code>",
			Head:    "",
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
